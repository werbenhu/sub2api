package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type multiGroupAPIKeyRepoStub struct {
	APIKeyRepository
	created *APIKey
}

func (s *multiGroupAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	key.ID = 1
	s.created = key
	return nil
}

type multiGroupUserRepoStub struct {
	UserRepository
}

func (multiGroupUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 42, Status: StatusActive}, nil
}

type multiGroupGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (s multiGroupGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	return s.groups[id], nil
}

type multiGroupSubscriptionRepoStub struct {
	UserSubscriptionRepository
	groupIDs []int64
}

func (s multiGroupSubscriptionRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	out := make([]UserSubscription, 0, len(s.groupIDs))
	for _, groupID := range s.groupIDs {
		out = append(out, UserSubscription{UserID: 42, GroupID: groupID, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)})
	}
	return out, nil
}

func (s multiGroupSubscriptionRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	return &UserSubscription{UserID: userID, GroupID: groupID, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestAPIKeyCreateDefaultsToAllActiveSubscriptionGroups(t *testing.T) {
	groups := map[int64]*Group{
		1: {ID: 1, Status: StatusActive},
		2: {ID: 2, Status: StatusActive},
	}
	keyRepo := &multiGroupAPIKeyRepoStub{}
	svc := NewAPIKeyService(keyRepo, multiGroupUserRepoStub{}, multiGroupGroupRepoStub{groups: groups}, multiGroupSubscriptionRepoStub{groupIDs: []int64{1, 2}}, nil, nil, &config.Config{})

	key, err := svc.Create(context.Background(), 42, CreateAPIKeyRequest{Name: "internal"})

	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, key.GroupIDs)
	require.Equal(t, int64(1), *key.GroupID)
	require.Equal(t, []int64{1, 2}, keyRepo.created.GroupIDs)
}

func TestAPIKeyResolveGroupForModelUsesExplicitRoute(t *testing.T) {
	groups := map[int64]*Group{
		1: {ID: 1, Platform: PlatformAnthropic, Status: StatusActive},
		2: {ID: 2, Platform: PlatformComposite, Status: StatusActive},
	}
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{
		ID: 1, GroupID: 2, PublicModel: "internal-shared", TargetPlatform: PlatformOpenAI, Enabled: true,
	}}})
	svc := NewAPIKeyService(nil, nil, multiGroupGroupRepoStub{groups: groups}, multiGroupSubscriptionRepoStub{}, nil, nil, nil)
	svc.SetCompositeRouteResolver(resolver)
	key := &APIKey{UserID: 42, GroupID: int64Pointer(1), GroupIDs: []int64{1, 2}}

	require.NoError(t, svc.ResolveGroupForModel(context.Background(), key, "internal-shared", CompositeRouteEndpointResponses))
	require.Equal(t, int64(2), *key.GroupID)
	require.Same(t, groups[2], key.Group)
}

func TestAPIKeyAuthSnapshotPreservesMultipleGroups(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, UserID: 42, GroupID: int64Pointer(1), GroupIDs: []int64{1, 2}, Key: "secret", User: &User{ID: 42, Status: StatusActive}}

	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	restored := svc.snapshotToAPIKey(key.Key, snapshot)

	require.Equal(t, apiKeyAuthSnapshotVersion, snapshot.Version)
	require.Equal(t, []int64{1, 2}, restored.GroupIDs)
	require.True(t, restored.HasMultipleGroups())
}

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type platformBindingGroupRepo struct {
	AdminGroupRepository
	group     *Group
	liteReads int
}

func (r *platformBindingGroupRepo) GetByIDLite(context.Context, int64) (*Group, error) {
	r.liteReads++
	return r.group, nil
}

func (r *platformBindingGroupRepo) ExistsByIDs(_ context.Context, ids []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

func TestGroupPlatformBindingsPreserveMixedScheduling(t *testing.T) {
	account := &Account{Platform: PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": true}}
	for _, platform := range []string{PlatformAnthropic, PlatformGemini} {
		group := &Group{Platform: platform}
		require.True(t, group.SupportsAccount(account))
		require.False(t, group.SupportsPlatform(PlatformAntigravity))
		repo := &platformBindingGroupRepo{group: group}
		require.NoError(t, validateAccountGroupPlatforms(context.Background(), repo, account, []int64{1, 1}))
		require.Equal(t, 1, repo.liteReads)
	}
	account.Extra = nil
	require.False(t, (&Group{Platform: PlatformAnthropic}).SupportsAccount(account))
	account.Extra = map[string]any{"mixed_scheduling": true}
	require.False(t, (&Group{Platform: PlatformComposite, Platforms: []string{PlatformAnthropic, PlatformGemini}}).SupportsAccount(account))
}

func TestGroupPlatformBindingsReturnSameBadRequestAcrossServices(t *testing.T) {
	for _, admin := range []bool{false, true} {
		repo := &platformBindingGroupRepo{group: &Group{ID: 1, Platform: PlatformComposite, Platforms: []string{PlatformOpenAI}}}
		var err error
		if admin {
			_, err = (&adminServiceImpl{groupRepo: repo}).CreateAccount(context.Background(), &CreateAccountInput{
				Name: "Grok", Platform: PlatformGrok, Type: AccountTypeAPIKey, Concurrency: 1,
				GroupIDs: []int64{1}, SkipDefaultGroupBind: true, SkipMixedChannelCheck: true,
			})
		} else {
			_, err = NewAccountService(nil, repo).Create(context.Background(), CreateAccountRequest{
				Name: "Grok", Platform: PlatformGrok, Type: AccountTypeAPIKey, GroupIDs: []int64{1},
			})
		}
		require.True(t, infraerrors.IsBadRequest(err), "admin=%v: %v", admin, err)
		require.Equal(t, "GROUP_PLATFORM_ACCOUNT_MISMATCH", infraerrors.Reason(err))
		require.Equal(t, 1, repo.liteReads)
	}
}

func TestGroupPlatformAuthSnapshotPreservesRestrictions(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{User: &User{}, Group: &Group{ID: 1, Platform: PlatformComposite, Platforms: []string{PlatformAnthropic, PlatformOpenAI}}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	restored := svc.snapshotToAPIKey("test", snapshot)
	require.Equal(t, key.Group.Platforms, restored.Group.Platforms)
	require.False(t, restored.Group.SupportsPlatform(PlatformGemini))
	restored.Group.Platforms[0] = PlatformGrok
	require.Equal(t, PlatformAnthropic, snapshot.Group.Platforms[0])

	_, used, err := svc.applyAuthCacheEntry("test", &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{Version: 28}})
	require.NoError(t, err)
	require.False(t, used)
}

func TestGroupPlatformPreResolvedTargetIsValidated(t *testing.T) {
	group := &Group{ID: 1, Platform: PlatformComposite, Platforms: []string{PlatformAnthropic, PlatformAntigravity}}
	repo := &platformBindingGroupRepo{group: group}
	svc := &GatewayService{groupRepo: repo}
	for _, providedGroup := range []*Group{group, nil} {
		ctx := WithResolvedTargetPlatform(context.Background(), PlatformGemini)
		_, _, err := svc.resolvePlatform(ctx, int64Pointer(1), providedGroup, "gemini-2.5-flash")
		require.True(t, infraerrors.IsBadRequest(err))
		ctx = WithResolvedTargetPlatform(context.Background(), PlatformAnthropic)
		platform, _, err := svc.resolvePlatform(ctx, int64Pointer(1), providedGroup, "claude-sonnet-4-5")
		require.NoError(t, err)
		require.Equal(t, PlatformAnthropic, platform)
	}
}

func TestGroupPlatformMultipleCompositeGroupsSelectCompatibleGroup(t *testing.T) {
	groups := map[int64]*Group{
		1: {ID: 1, Platform: PlatformComposite, Status: StatusActive, Platforms: []string{PlatformAnthropic, PlatformOpenAI}},
		2: {ID: 2, Platform: PlatformComposite, Status: StatusActive, Platforms: []string{PlatformGemini, PlatformGrok}},
	}
	for _, resolver := range []*CompositeRouteResolver{nil, NewCompositeRouteResolver(nil)} {
		svc := &APIKeyService{groupRepo: multiGroupGroupRepoStub{groups: groups}, compositeResolver: resolver}
		key := &APIKey{User: &User{}, GroupIDs: []int64{1, 2}}
		require.NoError(t, svc.ResolveGroupForModel(context.Background(), key, "gemini-2.5-flash", CompositeRouteEndpointGemini))
		require.Equal(t, int64(2), *key.GroupID)
		err := svc.ResolveGroupForModel(context.Background(), key, "deepseek-chat", CompositeRouteEndpointChatCompletions)
		require.True(t, infraerrors.IsBadRequest(err))
	}
}

func TestGroupPlatformsNormalizeEmptyArray(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformComposite} {
		_, platforms, err := normalizeGroupPlatforms(platform, nil)
		require.NoError(t, err)
		require.NotNil(t, platforms)
		require.Empty(t, platforms)
	}
}

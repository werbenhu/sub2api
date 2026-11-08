package service

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyQuotaUpdaterStub struct{}

func (apiKeyQuotaUpdaterStub) UpdateQuotaUsed(context.Context, int64, float64) error { return nil }
func (apiKeyQuotaUpdaterStub) UpdateRateLimitUsage(context.Context, int64, float64) error {
	return nil
}

type simpleModeUsageBillingRepoStub struct {
	UsageBillingRepository
	seen map[string]struct{}
	cmds []*UsageBillingCommand
}

func (s *simpleModeUsageBillingRepoStub) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	s.cmds = append(s.cmds, cmd)
	if s.seen == nil {
		s.seen = make(map[string]struct{})
	}
	key := cmd.RequestID + ":" + strconv.FormatInt(cmd.APIKeyID, 10)
	if _, ok := s.seen[key]; ok {
		return &UsageBillingApplyResult{Applied: false}, nil
	}
	s.seen[key] = struct{}{}
	return &UsageBillingApplyResult{Applied: true}, nil
}

func TestBuildUsageBillingCommandSimpleModeChargesSubscription(t *testing.T) {
	p := &postUsageBillingParams{
		Cost:          &CostBreakdown{ActualCost: 3.25, TotalCost: 2.5},
		User:          &User{ID: 7},
		APIKey:        &APIKey{ID: 13, Quota: 100, RateLimit5h: 10},
		Account:       &Account{ID: 9, Type: AccountTypeAPIKey},
		Subscription:  &UserSubscription{ID: 21},
		APIKeyService: &apiKeyQuotaUpdaterStub{},
	}

	cmd := buildUsageBillingCommand("simple-req", nil, p)
	require.NotNil(t, cmd)
	require.Zero(t, cmd.BalanceCost)
	require.Equal(t, 3.25, cmd.SubscriptionCost)
	require.Equal(t, int64(21), *cmd.SubscriptionID)
	require.Equal(t, 3.25, cmd.APIKeyQuotaCost)
	require.Equal(t, 3.25, cmd.APIKeyRateLimitCost)
}

func TestApplyUsageBillingSimpleModeDeduplicatesSubscriptionCharge(t *testing.T) {
	repo := &simpleModeUsageBillingRepoStub{}
	p := &postUsageBillingParams{
		Cost:          &CostBreakdown{ActualCost: 3.25, TotalCost: 3.25},
		User:          &User{ID: 7},
		APIKey:        &APIKey{ID: 13, RateLimit5h: 10},
		Account:       &Account{ID: 9, Type: AccountTypeAPIKey},
		Subscription:  &UserSubscription{ID: 21},
		APIKeyService: &apiKeyQuotaUpdaterStub{},
	}

	first, err := applyUsageBilling(context.Background(), "simple-req", nil, p, &billingDeps{
		deferredService:     &DeferredService{},
		billingCacheService: &BillingCacheService{},
	}, repo)
	require.NoError(t, err)
	require.True(t, first)
	second, err := applyUsageBilling(context.Background(), "simple-req", nil, p, &billingDeps{
		deferredService:     &DeferredService{},
		billingCacheService: &BillingCacheService{},
	}, repo)
	require.NoError(t, err)
	require.False(t, second)
	require.Len(t, repo.cmds, 2)
	for _, cmd := range repo.cmds {
		require.Zero(t, cmd.BalanceCost)
		require.Equal(t, 3.25, cmd.SubscriptionCost)
	}
}

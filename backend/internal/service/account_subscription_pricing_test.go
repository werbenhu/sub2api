//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAccountModelPricingOverridesAndOfficialFallback(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)
	input := 1e-6
	groupPrice := 9e-6
	group := &Group{ID: 1, LongContextPricingEnabled: true, ModelPricing: []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, InputPrice: &groupPrice}}}
	account := &Account{Platform: PlatformAnthropic, Extra: map[string]any{"model_pricing": []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, InputPrice: &input}}}}
	ctx := withAccountModelPricing(context.Background(), account)
	resolved := r.Resolve(ctx, PricingInput{Model: "claude-sonnet-4", Group: group})
	require.Equal(t, PricingSourceAccount, resolved.Source)
	require.InDelta(t, input, resolved.BasePricing.InputPricePerToken, 1e-12)
	// Unset output price still comes from the official catalog, not the group card.
	require.InDelta(t, 15e-6, resolved.BasePricing.OutputPricePerToken, 1e-12)
	cost, err := bs.CalculateTokenCostForRequest(TokenCostRequest{Ctx: ctx, Model: "claude-sonnet-4", Group: group, Resolver: r, Resolved: resolved, Tokens: UsageTokens{InputTokens: 1_000_000, OutputTokens: 1_000_000}, RateMultiplier: 2})
	require.NoError(t, err)
	require.InDelta(t, 32, cost.ActualCost, 1e-8)
	require.InDelta(t, 3e-6, bs.fallbackPrices["claude-sonnet-4"].InputPricePerToken, 1e-12)
	resolved = r.Resolve(withAccountModelPricing(ctx, nil), PricingInput{Model: "claude-sonnet-4", Group: group})
	require.Equal(t, PricingSourceGroup, resolved.Source)
	require.Error(t, validateAccountModelPricing(map[string]any{"model_pricing": "invalid"}))
	require.NoError(t, validateAccountModelPricing(map[string]any{"model_pricing": []any{}}))
}

type budgetReadRepo struct {
	UserSubscriptionRepository
	sub *UserSubscription
	err error
}

func (r budgetReadRepo) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return r.sub, r.err
}

func TestSubscriptionBudgetEligibilityUsesDatabaseAndResets(t *testing.T) {
	now := time.Now()
	start := now.Add(-7 * time.Hour)
	b := &domain.SubscriptionBudget{AmountUSD: 10, ResetPeriod: "hours", ResetHours: 6}
	sub := &UserSubscription{Status: SubscriptionStatusActive, StartsAt: start, ExpiresAt: now.Add(24 * time.Hour), Budget: b, BudgetUsageUSD: 10, BudgetWindowStart: &start}
	svc := &BillingCacheService{subRepo: budgetReadRepo{sub: sub}}
	require.NoError(t, svc.checkSubscriptionBudget(context.Background(), 1, 1, 0))
	window := start.Add(6 * time.Hour)
	sub.BudgetWindowStart = &window
	require.ErrorIs(t, svc.checkSubscriptionBudget(context.Background(), 1, 1, 0), ErrSubscriptionBudgetExceeded)
	sub.BudgetUsageUSD = 9
	require.NoError(t, svc.checkSubscriptionBudget(context.Background(), 1, 1, 1))
	require.ErrorIs(t, svc.checkSubscriptionBudget(context.Background(), 1, 1, 1.01), ErrSubscriptionBudgetExceeded)
	// Middleware must not use obsolete group quotas for a configured allowance.
	zero := 0.01
	group := &Group{DailyLimitUSD: &zero}
	sub.DailyUsageUSD = 100
	subSvc := &SubscriptionService{now: func() time.Time { return now }}
	_, err := subSvc.ValidateAndCheckLimits(sub, group)
	require.NoError(t, err)
	svc.subRepo = budgetReadRepo{err: errors.New("database unavailable")}
	require.ErrorIs(t, svc.checkSubscriptionBudget(context.Background(), 1, 1, 0), ErrBillingServiceUnavailable)
}

func TestSubscriptionBudgetUpdatePreservesConsumptionUntilExplicitReset(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	start := now.Add(-2 * time.Hour)
	stub := &resetQuotaUserSubRepoStub{sub: &UserSubscription{ID: 1, UserID: 10, GroupID: 20, StartsAt: start, ExpiresAt: now.Add(24 * time.Hour), Budget: &domain.SubscriptionBudget{AmountUSD: 10, ResetPeriod: "hours", ResetHours: 6}, BudgetUsageUSD: 7, BudgetWindowStart: &start}}
	svc := newResetQuotaSvc(stub)
	svc.now = func() time.Time { return now }
	budget := &domain.SubscriptionBudget{AmountUSD: 20, ResetPeriod: "hours", ResetHours: 12}
	sub, err := svc.UpdateBudget(context.Background(), 1, budget, false)
	require.NoError(t, err)
	require.Equal(t, 7.0, sub.BudgetUsageUSD)
	require.Equal(t, 13.0, sub.Budget.AmountUSD-sub.EffectiveBudgetUsage(now))
	sub, err = svc.UpdateBudget(context.Background(), 1, budget, true)
	require.NoError(t, err)
	require.Zero(t, sub.EffectiveBudgetUsage(now))
	require.True(t, subscriptionBudgetsEqual(budget, &domain.SubscriptionBudget{AmountUSD: 20, ResetPeriod: "hours", ResetHours: 12, ResetTime: "00:00", Timezone: "Asia/Shanghai", ResetDays: 1}))
}

package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func validateSubscriptionBudget(b *domain.SubscriptionBudget) error {
	if err := b.Validate(); err != nil {
		return infraerrors.BadRequest("SUBSCRIPTION_BUDGET_INVALID", err.Error())
	}
	return nil
}

func cloneSubscriptionBudget(b *domain.SubscriptionBudget) *domain.SubscriptionBudget {
	if b == nil {
		return nil
	}
	copy := *b
	return &copy
}

func subscriptionBudgetsEqual(a, b *domain.SubscriptionBudget) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.AmountUSD != b.AmountUSD || a.ResetPeriod != b.ResetPeriod {
		return false
	}
	switch a.ResetPeriod {
	case "none":
		return true
	case "hours":
		return a.ResetHours == b.ResetHours
	case "custom":
		if a.ResetDays != b.ResetDays {
			return false
		}
	}
	return a.ResetTime == b.ResetTime && a.Timezone == b.Timezone
}

// UpdateBudget preserves current-period consumption unless explicitly reset.
func (s *SubscriptionService) UpdateBudget(ctx context.Context, id int64, budget *domain.SubscriptionBudget, resetUsage bool) (*UserSubscription, error) {
	if err := validateSubscriptionBudget(budget); err != nil {
		return nil, err
	}
	var userID, groupID int64
	err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		sub, err := s.userSubRepo.GetByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		now := s.now()
		used := sub.EffectiveBudgetUsage(now)
		if resetUsage || sub.Budget == nil {
			used = 0
		}
		sub.Budget = cloneSubscriptionBudget(budget)
		sub.BudgetUsageUSD = used
		window := budget.WindowStart(sub.StartsAt, sub.ExpiresAt, now)
		sub.BudgetWindowStart = &window
		userID, groupID = sub.UserID, sub.GroupID
		return s.userSubRepo.Update(txCtx, sub)
	})
	if err != nil {
		return nil, err
	}
	if err := s.invalidateSubscriptionCaches(userID, groupID); err != nil {
		return nil, err
	}
	return s.userSubRepo.GetByID(ctx, id)
}

// Configured allowances read authoritative usage. Redis's asynchronous legacy
// daily/weekly/monthly counters must not allow a spent monetary allowance.
func (s *BillingCacheService) checkSubscriptionBudget(ctx context.Context, userID, groupID int64, pending float64) error {
	if s.subRepo == nil {
		return ErrBillingServiceUnavailable
	}
	sub, err := s.subRepo.GetByUserIDAndGroupID(ctx, userID, groupID)
	if err != nil {
		return ErrBillingServiceUnavailable.WithCause(err)
	}
	if sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(time.Now()) {
		return ErrSubscriptionInvalid
	}
	if !sub.CheckBudget(pending, time.Now()) {
		return subscriptionBudgetExceeded(sub, time.Now())
	}
	return nil
}

func subscriptionBudgetExceeded(sub *UserSubscription, now time.Time) error {
	if sub.Budget != nil {
		if next := sub.Budget.NextReset(sub.StartsAt, sub.ExpiresAt, now); next != nil {
			return ErrSubscriptionBudgetExceeded.WithMetadata(map[string]string{"window_resets_at": next.Format(time.RFC3339)})
		}
	}
	return ErrSubscriptionBudgetExceeded
}

// resetBudgetUsage restores the current allowance without moving its schedule.
// Both single and bulk admin resets use the same locked write as billing.
func (s *SubscriptionService) resetBudgetUsage(ctx context.Context, id int64) (*UserSubscription, error) {
	var userID, groupID int64
	err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		sub, err := s.userSubRepo.GetByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		window := sub.Budget.WindowStart(sub.StartsAt, sub.ExpiresAt, s.now())
		sub.BudgetUsageUSD = 0
		sub.BudgetWindowStart = &window
		userID, groupID = sub.UserID, sub.GroupID
		return s.userSubRepo.Update(txCtx, sub)
	})
	if err != nil {
		return nil, err
	}
	if err := s.invalidateSubscriptionCaches(userID, groupID); err != nil {
		return nil, err
	}
	return s.userSubRepo.GetByID(ctx, id)
}

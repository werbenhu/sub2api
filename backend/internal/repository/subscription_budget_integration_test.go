//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionBudgetSQLCalendarParity(t *testing.T) {
	starts := time.Date(2026, 1, 31, 8, 30, 0, 0, time.UTC)
	expires := starts.AddDate(1, 0, 0)
	for _, period := range []string{"hours", "daily", "weekly", "monthly", "custom", "none"} {
		for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York"} {
			for _, instant := range []string{"2026-02-02T02:00:00Z", "2026-03-08T07:00:00Z", "2026-03-08T07:30:00Z", "2026-11-01T05:45:00Z", "2026-11-01T06:45:00Z"} {
				t.Run(period+zone+instant, func(t *testing.T) {
					now, err := time.Parse(time.RFC3339, instant)
					require.NoError(t, err)
					budget := &domain.SubscriptionBudget{AmountUSD: 10, ResetPeriod: period, ResetDays: 3, ResetHours: 6, ResetTime: "02:30", Timezone: zone}
					data, err := json.Marshal(budget)
					require.NoError(t, err)
					var got time.Time
					err = integrationDB.QueryRowContext(context.Background(), "SELECT subscription_budget_window($1,$2,$3,$4)", string(data), starts, expires, now).Scan(&got)
					require.NoError(t, err)
					require.True(t, budget.WindowStart(starts, expires, now).Equal(got), "Go and SQL disagree: got %v want %v", got, budget.WindowStart(starts, expires, now))
				})
			}
		}
	}
}

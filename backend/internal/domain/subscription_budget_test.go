package domain

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionBudgetWindows(t *testing.T) {
	parse := func(s string) time.Time { v, err := time.Parse(time.RFC3339, s); require.NoError(t, err); return v }
	starts := parse("2026-01-31T08:30:00Z")
	expires := parse("2026-06-01T00:00:00Z")
	for _, tt := range []struct {
		name, period, now, window, next string
		hours, days                     int
	}{
		{"hourly anchor", "hours", "2026-02-01T01:00:00Z", "2026-01-31T20:30:00Z", "2026-02-01T02:30:00Z", 6, 0},
		{"hour boundary", "hours", "2026-01-31T14:30:00Z", "2026-01-31T14:30:00Z", "2026-01-31T20:30:00Z", 6, 0},
		{"daily before reset", "daily", "2026-02-02T01:59:59Z", "2026-02-01T02:00:00Z", "2026-02-02T02:00:00Z", 0, 0},
		{"weekly Monday", "weekly", "2026-02-02T02:00:00Z", "2026-02-02T02:00:00Z", "2026-02-09T02:00:00Z", 0, 0},
		{"calendar month", "monthly", "2026-03-01T02:00:00Z", "2026-03-01T02:00:00Z", "2026-04-01T02:00:00Z", 0, 0},
		{"custom days", "custom", "2026-02-06T02:00:00Z", "2026-02-06T02:00:00Z", "2026-02-09T02:00:00Z", 0, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := &SubscriptionBudget{AmountUSD: 10, ResetPeriod: tt.period, ResetTime: "10:00", Timezone: "Asia/Shanghai", ResetHours: tt.hours, ResetDays: tt.days}
			require.NoError(t, b.Validate())
			require.Equal(t, parse(tt.window), b.WindowStart(starts, expires, parse(tt.now)).UTC())
			require.Equal(t, parse(tt.next), b.NextReset(starts, expires, parse(tt.now)).UTC())
		})
	}
}

func TestSubscriptionBudgetExpiryAndDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	starts := time.Date(2026, 3, 7, 12, 0, 0, 0, loc)
	expires := starts.AddDate(0, 0, 2)
	b := &SubscriptionBudget{AmountUSD: 10, ResetPeriod: "daily", ResetTime: "10:00", Timezone: loc.String()}
	now := time.Date(2026, 3, 8, 10, 0, 0, 0, loc)
	require.True(t, b.WindowStart(starts, expires, now).Equal(now))
	require.Equal(t, 23*time.Hour, now.Sub(time.Date(2026, 3, 7, 10, 0, 0, 0, loc)))
	require.Nil(t, b.NextReset(starts, expires, expires))
	require.True(t, b.WindowStart(starts, expires, expires.AddDate(0, 0, 5)).Before(expires))
	b.ResetPeriod = "hours"
	b.ResetHours = 24
	require.True(t, b.WindowStart(starts, expires, starts.Add(24*time.Hour)).Equal(starts.Add(24*time.Hour)))
}

func TestSubscriptionBudgetValidation(t *testing.T) {
	for _, b := range []*SubscriptionBudget{
		{AmountUSD: 0}, {AmountUSD: math.NaN()}, {AmountUSD: math.Inf(1)},
		{AmountUSD: 1, ResetPeriod: "hours", ResetHours: 0},
		{AmountUSD: 1, ResetPeriod: "custom", ResetDays: -1},
		{AmountUSD: 1, ResetPeriod: "daily", ResetTime: "25:00", Timezone: "UTC"},
		{AmountUSD: 1, ResetPeriod: "daily", ResetTime: "00:00", Timezone: "invalid"},
	} {
		require.Error(t, b.Validate())
	}
	var legacy *SubscriptionBudget
	require.NoError(t, legacy.Validate())
}

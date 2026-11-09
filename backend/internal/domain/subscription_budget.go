package domain

import (
	"fmt"
	"math"
	"time"
)

// SubscriptionBudget describes a renewable USD allowance, independent of the
// subscription's total validity. Unspent allowance never carries over.
type SubscriptionBudget struct {
	AmountUSD   float64 `json:"amount_usd"`
	ResetPeriod string  `json:"reset_period"`
	ResetDays   int     `json:"reset_days,omitempty"`
	ResetHours  int     `json:"reset_hours,omitempty"`
	ResetTime   string  `json:"reset_time"`
	Timezone    string  `json:"timezone"`
}

func (b *SubscriptionBudget) Validate() error {
	if b == nil {
		return nil
	}
	if math.IsNaN(b.AmountUSD) || math.IsInf(b.AmountUSD, 0) || b.AmountUSD <= 0 {
		return fmt.Errorf("allowance must be a positive USD amount")
	}
	switch b.ResetPeriod {
	case "none":
		return nil
	case "hours":
		if b.ResetHours < 1 || b.ResetHours > 876000 {
			return fmt.Errorf("reset hours must be between 1 and 876000")
		}
		return nil
	case "daily", "weekly", "monthly":
	case "custom":
		if b.ResetDays < 1 || b.ResetDays > 36500 {
			return fmt.Errorf("custom reset days must be between 1 and 36500")
		}
	default:
		return fmt.Errorf("invalid allowance reset period")
	}
	if _, err := time.Parse("15:04", b.ResetTime); err != nil {
		return fmt.Errorf("reset time must be HH:mm")
	}
	if _, err := time.LoadLocation(b.Timezone); err != nil || b.Timezone == "" {
		return fmt.Errorf("invalid reset timezone")
	}
	return nil
}

func (b *SubscriptionBudget) boundaries(startsAt, now time.Time) (time.Time, time.Time) {
	if b.ResetPeriod == "hours" {
		period := time.Duration(max(b.ResetHours, 1)) * time.Hour
		window := startsAt.Add(now.Sub(startsAt) / period * period)
		return window, window.Add(period)
	}
	loc, err := time.LoadLocation(b.Timezone)
	if err != nil {
		loc = time.UTC
	}
	clock, _ := time.Parse("15:04", b.ResetTime)
	// PostgreSQL resolves an ambiguous local clock using the later occurrence,
	// and moves a nonexistent clock forward across the DST gap.
	resetAt := func(year int, month time.Month, day int) time.Time {
		wall := time.Date(year, month, day, clock.Hour(), clock.Minute(), 0, 0, time.UTC)
		noon := time.Date(year, month, day, 12, 0, 0, 0, loc)
		_, before := noon.Add(-36 * time.Hour).Zone()
		_, after := noon.Add(36 * time.Hour).Zone()
		first, second := wall.Add(-time.Duration(before)*time.Second), wall.Add(-time.Duration(after)*time.Second)
		matches := func(v time.Time) bool {
			local := v.In(loc)
			return local.Year() == wall.Year() && local.Month() == wall.Month() && local.Day() == wall.Day() && local.Hour() == clock.Hour() && local.Minute() == clock.Minute()
		}
		if matches(first) && !matches(second) {
			return first.In(loc)
		}
		if matches(second) && !matches(first) {
			return second.In(loc)
		}
		if second.After(first) {
			return second.In(loc)
		}
		return first.In(loc)
	}
	add := func(v time.Time, months, days int) time.Time {
		v = v.In(loc)
		return resetAt(v.Year(), v.Month()+time.Month(months), v.Day()+days)
	}
	t := now.In(loc)
	window := resetAt(t.Year(), t.Month(), t.Day())
	switch b.ResetPeriod {
	case "weekly":
		window = resetAt(t.Year(), t.Month(), t.Day()-(int(t.Weekday())+6)%7)
		if now.Before(window) {
			window = add(window, 0, -7)
		}
		return window, add(window, 0, 7)
	case "monthly":
		window = resetAt(t.Year(), t.Month(), 1)
		if now.Before(window) {
			window = add(window, -1, 0)
		}
		return window, add(window, 1, 0)
	case "custom":
		anchor := startsAt.In(loc)
		anchor = resetAt(anchor.Year(), anchor.Month(), anchor.Day())
		if startsAt.Before(anchor) {
			anchor = add(anchor, 0, -1)
		}
		date := func(v time.Time) time.Time { return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC) }
		days := int(date(t).Sub(date(anchor)).Hours() / 24)
		period := max(b.ResetDays, 1)
		window = add(anchor, 0, days/period*period)
		if now.Before(window) {
			window = add(window, 0, -period)
		}
		return window, add(window, 0, period)
	default:
		if now.Before(window) {
			window = add(window, 0, -1)
		}
		return window, add(window, 0, 1)
	}

}

func (b *SubscriptionBudget) WindowStart(startsAt, expiresAt, now time.Time) time.Time {
	if b == nil || b.ResetPeriod == "none" {
		return startsAt
	}
	if !now.Before(expiresAt) {
		now = expiresAt.Add(-time.Microsecond)
	}
	window, _ := b.boundaries(startsAt, now)
	if window.Before(startsAt) {
		return startsAt
	}
	return window
}

func (b *SubscriptionBudget) NextReset(startsAt, expiresAt, now time.Time) *time.Time {
	if b == nil || b.ResetPeriod == "none" || !now.Before(expiresAt) {
		return nil
	}
	_, next := b.boundaries(startsAt, now)
	if !next.Before(expiresAt) {
		return nil
	}
	return &next
}

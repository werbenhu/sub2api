package repository

// Reset and consume in the same row update, including idempotent billing transactions.
const incrementSubscriptionUsageSQL = `
 UPDATE user_subscriptions us SET
 daily_usage_usd = us.daily_usage_usd + $1,
 weekly_usage_usd = us.weekly_usage_usd + $1,
 monthly_usage_usd = us.monthly_usage_usd + $1,
 budget_usage_usd = CASE
 WHEN us.budget_window_start IS NULL OR us.budget_window_start < subscription_budget_window(us.budget, us.starts_at, us.expires_at, NOW()) THEN $1
 ELSE us.budget_usage_usd + $1 END,
 budget_window_start = GREATEST(us.budget_window_start, subscription_budget_window(us.budget, us.starts_at, us.expires_at, NOW())),
 updated_at = NOW()
 FROM groups g WHERE us.id = $2 AND us.deleted_at IS NULL AND us.group_id = g.id AND g.deleted_at IS NULL
`

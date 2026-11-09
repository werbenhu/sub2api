-- NULL budgets retain the existing group quota behavior.
ALTER TABLE subscription_plans ADD COLUMN IF NOT EXISTS budget jsonb;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS budget jsonb;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS budget_usage_usd decimal(20,10) NOT NULL DEFAULT 0;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS budget_window_start timestamptz;

ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS subscription_budget jsonb;

-- Shared by both billing writers. Calendar boundaries match domain.SubscriptionBudget.
CREATE OR REPLACE FUNCTION subscription_budget_window(b jsonb, starts timestamptz, expires timestamptz, at_time timestamptz)
RETURNS timestamptz LANGUAGE plpgsql STABLE AS $$
DECLARE
  local_now timestamp;
  local_start timestamp;
  boundary timestamp;
  anchor timestamp;
  zone text;
  clock_time time;
  period text;
  days integer;
BEGIN
  IF b IS NULL OR b = 'null'::jsonb OR b->>'reset_period' = 'none' THEN RETURN starts; END IF;
  IF b->>'reset_period' = 'hours' THEN
    RETURN GREATEST(starts, starts + floor(EXTRACT(EPOCH FROM (LEAST(at_time, expires - interval '1 microsecond') - starts)) / ((b->>'reset_hours')::integer * 3600)) * (b->>'reset_hours')::integer * interval '1 hour');
  END IF;
  zone := b->>'timezone';
  clock_time := (b->>'reset_time')::time;
  period := b->>'reset_period';
  local_now := LEAST(at_time, expires - interval '1 microsecond') AT TIME ZONE zone;
  local_start := starts AT TIME ZONE zone;
  IF period = 'weekly' THEN
    boundary := date_trunc('week', local_now)::date + clock_time;
    IF (boundary AT TIME ZONE zone) > LEAST(at_time, expires - interval '1 microsecond') THEN boundary := boundary - interval '7 days'; END IF;
  ELSIF period = 'monthly' THEN
    boundary := date_trunc('month', local_now)::date + clock_time;
    IF (boundary AT TIME ZONE zone) > LEAST(at_time, expires - interval '1 microsecond') THEN boundary := boundary - interval '1 month'; END IF;
  ELSIF period = 'custom' THEN
    days := (b->>'reset_days')::integer;
    anchor := local_start::date + clock_time;
    IF anchor > local_start THEN anchor := anchor - interval '1 day'; END IF;
    boundary := anchor + (floor((local_now::date - anchor::date)::numeric / days) * days)::integer * interval '1 day';
    IF (boundary AT TIME ZONE zone) > LEAST(at_time, expires - interval '1 microsecond') THEN boundary := boundary - days * interval '1 day'; END IF;
  ELSE
    boundary := local_now::date + clock_time;
    IF (boundary AT TIME ZONE zone) > LEAST(at_time, expires - interval '1 microsecond') THEN boundary := boundary - interval '1 day'; END IF;
  END IF;
  RETURN GREATEST(starts, boundary AT TIME ZONE zone);
END;
$$;

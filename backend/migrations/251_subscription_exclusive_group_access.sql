SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

INSERT INTO user_allowed_groups (user_id, group_id, created_at)
SELECT us.user_id, us.group_id, NOW()
FROM user_subscriptions AS us
JOIN groups AS g ON g.id = us.group_id
WHERE us.deleted_at IS NULL
  AND us.status = 'active'
  AND us.expires_at > NOW()
  AND g.deleted_at IS NULL
  AND g.is_exclusive = TRUE
ON CONFLICT (user_id, group_id) DO NOTHING;

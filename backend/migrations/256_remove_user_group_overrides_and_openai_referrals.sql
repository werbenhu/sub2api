-- Remove retired per-user group billing and RPM configuration.
DROP TABLE IF EXISTS user_group_rate_multipliers;

-- Remove account invitation snapshots left by earlier versions.
UPDATE accounts
SET extra = extra - 'codex_referral_snapshot'
WHERE extra ? 'codex_referral_snapshot';

-- Keep existing scoped keys editable after retiring invitation permissions.
UPDATE scoped_admin_api_keys
SET policy = jsonb_set(policy, '{rules}', (
    SELECT COALESCE(jsonb_agg(
        CASE WHEN jsonb_typeof(rule -> 'permissions') = 'array' THEN
            jsonb_set(rule, '{permissions}',
                (rule -> 'permissions') - 'openai.referrals.read' - 'openai.referrals.invite')
        ELSE rule END
        ORDER BY ordinal
    ), '[]'::jsonb)
    FROM jsonb_array_elements(policy -> 'rules') WITH ORDINALITY AS rules(rule, ordinal)
)), updated_at = NOW()
WHERE jsonb_typeof(policy -> 'rules') = 'array'
  AND policy -> 'rules' @? '$[*].permissions[*] ? (@ == "openai.referrals.read" || @ == "openai.referrals.invite")';

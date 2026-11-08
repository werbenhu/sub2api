SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE api_keys
ADD COLUMN IF NOT EXISTS group_ids JSONB;

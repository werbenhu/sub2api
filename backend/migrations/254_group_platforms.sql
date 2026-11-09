SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE groups
ADD COLUMN IF NOT EXISTS platforms JSONB NOT NULL DEFAULT '[]'::jsonb;

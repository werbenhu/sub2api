SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

UPDATE groups
SET subscription_type = 'subscription'
WHERE subscription_type <> 'subscription';

-- Copy channel pricing into groups that do not yet define their own prices.
-- to_jsonb keeps newly added pricing fields compatible without another backfill.
UPDATE groups AS g
SET model_pricing = (
    SELECT COALESCE(
        jsonb_agg(
            (to_jsonb(cmp) - 'id' - 'channel_id' - 'created_at' - 'updated_at') ||
            jsonb_build_object(
                'intervals',
                COALESCE((
                    SELECT jsonb_agg(
                        to_jsonb(cpi) - 'id' - 'pricing_id' - 'created_at' - 'updated_at'
                        ORDER BY cpi.sort_order, cpi.id
                    )
                    FROM channel_pricing_intervals AS cpi
                    WHERE cpi.pricing_id = cmp.id
                ), '[]'::jsonb)
            )
            ORDER BY cmp.id
        ),
        '[]'::jsonb
    )
    FROM channel_groups AS cg
    JOIN channels AS c ON c.id = cg.channel_id
    JOIN channel_model_pricing AS cmp ON cmp.channel_id = c.id
    WHERE cg.group_id = g.id
      AND c.status = 'active'
)
WHERE (g.model_pricing IS NULL OR g.model_pricing = '[]'::jsonb)
  AND EXISTS (
      SELECT 1
      FROM channel_groups AS cg
      JOIN channels AS c ON c.id = cg.channel_id
      JOIN channel_model_pricing AS cmp ON cmp.channel_id = c.id
      WHERE cg.group_id = g.id
        AND c.status = 'active'
  );

ALTER TABLE batch_image_jobs
ADD COLUMN IF NOT EXISTS subscription_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_batch_image_jobs_subscription_id
ON batch_image_jobs (subscription_id)
WHERE subscription_id IS NOT NULL;

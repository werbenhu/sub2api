//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration250UsesSubscriptionsAndBackfillsGroupPricing(t *testing.T) {
	content, err := FS.ReadFile("250_subscription_only_group_pricing.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "SET subscription_type = 'subscription'")
	require.Contains(t, sql, "FROM channel_groups AS cg")
	require.Contains(t, sql, "to_jsonb(cmp)")
	require.Contains(t, sql, "FROM channel_pricing_intervals AS cpi")
	require.Contains(t, sql, "g.model_pricing = '[]'::jsonb")
	require.Contains(t, sql, "ALTER TABLE batch_image_jobs")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS subscription_id BIGINT")
}

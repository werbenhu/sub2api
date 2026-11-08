//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration251BackfillsExclusiveGroupAccessForActiveSubscriptions(t *testing.T) {
	content, err := FS.ReadFile("251_subscription_exclusive_group_access.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "INSERT INTO user_allowed_groups (user_id, group_id, created_at)")
	require.Contains(t, sql, "FROM user_subscriptions AS us")
	require.Contains(t, sql, "us.status = 'active'")
	require.Contains(t, sql, "us.expires_at > NOW()")
	require.Contains(t, sql, "g.is_exclusive = TRUE")
	require.Contains(t, sql, "ON CONFLICT (user_id, group_id) DO NOTHING")
}

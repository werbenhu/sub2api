//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration253AddsOrderedAPIKeyGroups(t *testing.T) {
	content, err := FS.ReadFile("253_api_key_group_ids.sql")
	require.NoError(t, err)
	require.Contains(t, string(content), "ALTER TABLE api_keys")
	require.Contains(t, string(content), "ADD COLUMN IF NOT EXISTS group_ids JSONB")
}

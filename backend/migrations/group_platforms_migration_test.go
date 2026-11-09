package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupPlatformsMigration(t *testing.T) {
	content, err := FS.ReadFile("254_group_platforms.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE groups ADD COLUMN IF NOT EXISTS platforms JSONB NOT NULL DEFAULT '[]'::jsonb")
}

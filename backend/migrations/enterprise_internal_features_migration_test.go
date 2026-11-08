//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration252DisablesPublicBillingAndLegacyAuthenticationFeatures(t *testing.T) {
	content, err := FS.ReadFile("252_enterprise_internal_features.sql")
	require.NoError(t, err)
	sql := string(content)
	for _, key := range []string{
		"turnstile_enabled", "tencent_captcha_enabled", "aliyun_captcha_enabled",
		"dingtalk_connect_enabled", "oidc_connect_enabled", "balance_low_notify_enabled",
		"channel_monitor_enabled", "channel_monitor_user_visible", "available_channels_enabled", "payment_enabled",
		"totp_enabled", "passkey_enabled", "step_up_enabled", "github_oauth_enabled", "google_oauth_enabled",
	} {
		require.Contains(t, sql, "'"+key+"'")
	}
	require.Contains(t, sql, "UPDATE settings")
	require.Contains(t, sql, "SET value = 'false'")
	require.Contains(t, sql, "UPDATE users")
	require.Contains(t, sql, "SET totp_enabled = FALSE")
}

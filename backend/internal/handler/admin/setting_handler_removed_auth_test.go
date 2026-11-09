package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisableRemovedAuthenticationSettings(t *testing.T) {
	enabled := true
	req := &UpdateSettingsRequest{
		TotpEnabled: true, PasskeyEnabled: &enabled, StepUpEnabled: &enabled,
		TurnstileEnabled: true, TencentCaptchaEnabled: true, AliyunCaptchaEnabled: true,
		LinuxDoConnectEnabled: true, DingTalkConnectEnabled: true, WeChatConnectEnabled: true,
		OIDCConnectEnabled: true, GitHubOAuthEnabled: true, GoogleOAuthEnabled: true,
	}

	disableRemovedAuthenticationSettings(req)

	require.False(t, req.TotpEnabled)
	require.NotNil(t, req.PasskeyEnabled)
	require.False(t, *req.PasskeyEnabled)
	require.NotNil(t, req.StepUpEnabled)
	require.False(t, *req.StepUpEnabled)
	require.True(t, req.TurnstileEnabled)
	require.True(t, req.TencentCaptchaEnabled)
	require.True(t, req.AliyunCaptchaEnabled)
	require.False(t, req.LinuxDoConnectEnabled || req.DingTalkConnectEnabled || req.WeChatConnectEnabled)
	require.False(t, req.OIDCConnectEnabled || req.GitHubOAuthEnabled || req.GoogleOAuthEnabled)
}

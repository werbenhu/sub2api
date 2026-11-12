package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newKeyBillingHandler() *GatewayHandler { return &GatewayHandler{} }

func newKeyBillingContext(apiKey *service.APIKey) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/sub2api/billing", nil)
	if apiKey != nil {
		c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	}
	return c, w
}

func TestGatewayHandlerKeyBillingInfoUsesGroupRate(t *testing.T) {
	groupID := int64(7)
	apiKey := &service.APIKey{
		UserID:  11,
		GroupID: &groupID,
		Key:     "sk-sensitive-value",
		Group: &service.Group{
			ID:             groupID,
			Name:           "private-group-name",
			RateMultiplier: 0.75,
		},
	}
	c, w := newKeyBillingContext(apiKey)

	newKeyBillingHandler().KeyBillingInfo(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var got keyBillingInfoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "sub2api.key_billing", got.Object)
	require.Equal(t, 1, got.SchemaVersion)
	require.Equal(t, "token", got.BillingScope)
	require.Equal(t, 0.75, got.GroupRateMultiplier)
	require.Equal(t, 0.75, got.ResolvedRateMultiplier)
	require.False(t, got.PeakRateEnabled)
	require.Nil(t, got.PeakStart)
	require.Nil(t, got.PeakEnd)
	require.Nil(t, got.PeakRateMultiplier)
	require.Nil(t, got.AppliedPeakMultiplier)
	require.Equal(t, 0.75, got.EffectiveRateMultiplier)
	require.Nil(t, got.Timezone)
	require.False(t, got.ObservedAt.IsZero())
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fields))
	require.NotContains(t, fields, "user_rate_multiplier")
	require.NotContains(t, fields, "peak_start")
	require.NotContains(t, fields, "peak_end")
	require.NotContains(t, fields, "peak_rate_multiplier")
	require.NotContains(t, fields, "applied_peak_multiplier")
	require.NotContains(t, fields, "timezone")
	require.NotContains(t, w.Body.String(), apiKey.Key)
	require.NotContains(t, w.Body.String(), apiKey.Group.Name)
}

func TestBuildKeyBillingInfoAppliesPeakMultiplier(t *testing.T) {
	groupID := int64(7)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:                 groupID,
			RateMultiplier:     1.2,
			SubscriptionType:   service.SubscriptionTypeSubscription,
			PeakRateEnabled:    true,
			PeakStart:          "09:00",
			PeakEnd:            "18:00",
			PeakRateMultiplier: 1.5,
		},
	}
	now := time.Date(2026, time.July, 12, 10, 0, 0, 0, timezone.Location())

	got := buildKeyBillingInfo(apiKey, now)

	require.Equal(t, 1.2, got.GroupRateMultiplier)
	require.Equal(t, 1.2, got.ResolvedRateMultiplier)
	require.True(t, got.PeakRateEnabled)
	require.NotNil(t, got.PeakStart)
	require.Equal(t, "09:00", *got.PeakStart)
	require.NotNil(t, got.PeakEnd)
	require.Equal(t, "18:00", *got.PeakEnd)
	require.NotNil(t, got.PeakRateMultiplier)
	require.Equal(t, 1.5, *got.PeakRateMultiplier)
	require.NotNil(t, got.AppliedPeakMultiplier)
	require.Equal(t, 1.5, *got.AppliedPeakMultiplier)
	require.InDelta(t, 1.8, got.EffectiveRateMultiplier, 1e-12)
	require.NotNil(t, got.Timezone)
	require.Equal(t, timezone.Location().String(), *got.Timezone)
	require.Equal(t, now.UTC(), got.ObservedAt)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	for _, field := range []string{
		"peak_start",
		"peak_end",
		"peak_rate_multiplier",
		"applied_peak_multiplier",
		"timezone",
	} {
		require.Contains(t, fields, field)
	}
}

func TestKeyBillingInfoJSONKeepsZeroPeakMultiplierWhenEnabled(t *testing.T) {
	groupID := int64(7)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:                 groupID,
			SubscriptionType:   service.SubscriptionTypeSubscription,
			PeakRateEnabled:    true,
			PeakStart:          "00:00",
			PeakEnd:            "23:59",
			PeakRateMultiplier: 0,
		},
	}
	now := time.Date(2026, time.July, 12, 12, 0, 0, 0, timezone.Location())
	encoded, err := json.Marshal(buildKeyBillingInfo(apiKey, now))
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.JSONEq(t, "0", string(fields["peak_rate_multiplier"]))
	require.JSONEq(t, "0", string(fields["applied_peak_multiplier"]))
}

func TestGatewayHandlerKeyBillingInfoErrorsAreSafe(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		c, w := newKeyBillingContext(nil)
		newKeyBillingHandler().KeyBillingInfo(c)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("ungrouped API key", func(t *testing.T) {
		c, w := newKeyBillingContext(&service.APIKey{})
		newKeyBillingHandler().KeyBillingInfo(c)
		require.Equal(t, http.StatusForbidden, w.Code)
	})

}

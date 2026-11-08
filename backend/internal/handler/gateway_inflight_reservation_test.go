package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestMaxOutputTokens(t *testing.T) {
	require.Equal(t, 1024, requestMaxOutputTokens([]byte(`{"max_tokens":1024}`)))
	require.Equal(t, 2048, requestMaxOutputTokens([]byte(`{"max_completion_tokens":2048}`)))
	require.Equal(t, 4096, requestMaxOutputTokens([]byte(`{"max_output_tokens":4096}`)))
	require.Equal(t, 512, requestMaxOutputTokens([]byte(`{"generationConfig":{"maxOutputTokens":512}}`)))
	require.Equal(t, 0, requestMaxOutputTokens([]byte(`{"max_tokens":"x"}`)))
	require.Equal(t, 0, requestMaxOutputTokens([]byte(`{}`)))
}

type countingEstimator struct {
	calls  int
	cost   float64
	priced bool
}

func (e *countingEstimator) EstimateInflightReservation(context.Context, *service.APIKey, service.InflightEstimateRequest) (float64, bool) {
	e.calls++
	return e.cost, e.priced
}

func newInflightTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c
}

func TestReserveInflightBalance_SkipsWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	billing := service.NewBillingCacheService(nil, nil, activeSubscriptionRepoStub{}, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	est := &countingEstimator{cost: 1, priced: true}
	apiKey := &service.APIKey{User: &service.User{ID: 1}}

	done, err := reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, nil, tokenInflightEstimate("m", []byte(`{}`)))
	require.NoError(t, err)
	done()
	require.Zero(t, est.calls, "disabled switch must not even estimate")
}

func TestReserveInflightBalance_UnpricedFailOpenByDefaultFailClosedOptIn(t *testing.T) {
	cache := newHandlerInflightCache(10)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	billing := service.NewBillingCacheService(cache, nil, activeSubscriptionRepoStub{}, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	apiKey := &service.APIKey{User: &service.User{ID: 1}}
	est := &countingEstimator{priced: false}

	done, err := reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, nil, tokenInflightEstimate("unknown", nil))
	require.NoError(t, err)
	done()

	cfg.Billing.InflightReservation.FailClosedOnUnpriced = true
	_, err = reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, nil, tokenInflightEstimate("unknown", nil))
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
}

// handlerInflightCache 内存版余额缓存 + 在途预留（语义同 Redis Lua）。
type handlerInflightCache struct {
	service.BillingCache
	mu      sync.Mutex
	balance float64
	res     map[string]float64
}

func newHandlerInflightCache(balance float64) *handlerInflightCache {
	return &handlerInflightCache{balance: balance, res: map[string]float64{}}
}

type handlerSubscriptionCache struct {
	*handlerInflightCache
	subscription *service.SubscriptionCacheData
	scopes       map[string]map[string]float64
}

func (c *handlerSubscriptionCache) GetSubscriptionCache(context.Context, int64, int64) (*service.SubscriptionCacheData, error) {
	return c.subscription, nil
}

func (c *handlerSubscriptionCache) SetSubscriptionCache(_ context.Context, _ int64, _ int64, data *service.SubscriptionCacheData) error {
	c.subscription = data
	return nil
}

func (c *handlerSubscriptionCache) ReserveInflightScope(_ context.Context, scope, requestID string, amount float64, _ time.Duration) (float64, error) {
	if c.scopes == nil {
		c.scopes = make(map[string]map[string]float64)
	}
	requests := c.scopes[scope]
	if requests == nil {
		requests = make(map[string]float64)
		c.scopes[scope] = requests
	}
	pending := 0.0
	for _, value := range requests {
		pending += value
	}
	requests[requestID] = amount
	return pending, nil
}

func (c *handlerSubscriptionCache) ReleaseInflightScope(_ context.Context, scope, requestID string) error {
	requests := c.scopes[scope]
	delete(requests, requestID)
	if len(requests) == 0 {
		delete(c.scopes, scope)
	}
	return nil
}

func (m *handlerInflightCache) GetUserBalance(context.Context, int64) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balance, nil
}

func (m *handlerInflightCache) GetSubscriptionCache(context.Context, int64, int64) (*service.SubscriptionCacheData, error) {
	return &service.SubscriptionCacheData{Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (m *handlerInflightCache) SetSubscriptionCache(context.Context, int64, int64, *service.SubscriptionCacheData) error {
	return nil
}

func (m *handlerInflightCache) UpdateSubscriptionUsage(context.Context, int64, int64, float64) error {
	return nil
}

func (m *handlerInflightCache) GetUserPlatformQuotaCache(context.Context, int64, string) (*service.UserPlatformQuotaCacheEntry, bool, error) {
	return nil, false, nil
}

func (m *handlerInflightCache) ReserveInflightBalance(_ context.Context, _ int64, id string, amount, balance float64, _ time.Duration) (bool, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sum := 0.0
	for _, v := range m.res {
		sum += v
	}
	if len(m.res) > 0 && balance-sum < amount {
		return false, sum, nil
	}
	m.res[id] = amount
	return true, sum, nil
}

func (m *handlerInflightCache) ReleaseInflightBalance(_ context.Context, _ int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.res, id)
	return nil
}

func (m *handlerInflightCache) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.res)
}

func TestWrapUsageRecordTaskContext_HandsReservationToBillingTask(t *testing.T) {
	cache := &handlerSubscriptionCache{handlerInflightCache: newHandlerInflightCache(1)}
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	billing := service.NewBillingCacheService(cache, nil, activeSubscriptionRepoStub{}, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	limit := 10.0
	apiKey := &service.APIKey{User: &service.User{ID: 5}, Group: &service.Group{ID: 7, DailyLimitUSD: &limit}}
	subscription := &service.UserSubscription{ID: 1, UserID: 5, GroupID: 7}
	require.NoError(t, cache.SetSubscriptionCache(context.Background(), 5, 7, &service.SubscriptionCacheData{
		Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}))
	scopeCount := func() int { return len(cache.scopes["sub:1"]) }

	c := newInflightTestGinContext()
	done, err := reserveInflightBalance(c, billing, &countingEstimator{cost: 0.9, priced: true}, apiKey, subscription, tokenInflightEstimate("m", nil))
	require.NoError(t, err)
	require.Equal(t, 1, scopeCount())

	ran := false
	task, abandon := wrapUsageRecordTaskContext(c.Request.Context(), func(context.Context) { ran = true })
	done()
	require.Equal(t, 1, scopeCount(), "reservation held until the billing task finishes")
	task(context.Background())
	require.True(t, ran)
	require.Zero(t, scopeCount())
	abandon()

	c2 := newInflightTestGinContext()
	done2, err := reserveInflightBalance(c2, billing, &countingEstimator{cost: 0.9, priced: true}, apiKey, subscription, tokenInflightEstimate("m", nil))
	require.NoError(t, err)
	_, abandon2 := wrapUsageRecordTaskContext(c2.Request.Context(), func(context.Context) {})
	done2()
	require.Equal(t, 1, scopeCount())
	abandon2()
	require.Zero(t, scopeCount())
}

// 独立 web_search：在途预留超过订阅日额度时拒绝，且不残留预留。
func TestWebSearch_RejectsWhenInflightExceedsSubscriptionLimit(t *testing.T) {
	cache := &handlerSubscriptionCache{handlerInflightCache: newHandlerInflightCache(1.5)}
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	billingCache := service.NewBillingCacheService(cache, nil, activeSubscriptionRepoStub{}, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	billing := service.NewBillingService(cfg, nil)
	gw := service.NewGatewayService(
		nil, nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil, billing, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, service.NewModelPricingResolver(nil, billing), nil, nil, nil,
	)
	h := &GatewayHandler{gatewayService: gw, billingCacheService: billingCache}

	groupID := int64(3)
	perK := 1000.0
	dailyLimit := 1.5
	apiKey := &service.APIKey{
		ID: 9, User: &service.User{ID: 42}, GroupID: &groupID,
		Group: &service.Group{ID: groupID, Platform: service.PlatformGrok, RateMultiplier: 1, SearchPricePer1k: &perK, DailyLimitUSD: &dailyLimit},
	}
	subscription := &service.UserSubscription{ID: 8, UserID: 42, GroupID: groupID}
	require.NoError(t, cache.SetSubscriptionCache(context.Background(), 42, groupID, &service.SubscriptionCacheData{
		Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}))

	held, err := billingCache.ReserveFixedCharge(context.Background(), apiKey.User, apiKey, apiKey.Group, subscription, 1.0)
	require.NoError(t, err)
	defer held.HandlerDone()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/web_search", bytes.NewBufferString(`{"query":"sub2api"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeySubscription), subscription)

	h.WebSearch(c)

	require.NotEqual(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "daily")
	require.Len(t, cache.scopes, 1, "rejected request must not leave a reservation behind")
}

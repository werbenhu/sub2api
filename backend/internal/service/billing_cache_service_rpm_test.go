//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// userRPMCacheStub 记录每种计数器被调用的次数，并可注入返回值与错误。
type userRPMCacheStub struct {
	userGroupCalls int32
	userCalls      int32

	userGroupCounts []int // 依次返回的计数值
	userGroupErr    error
	userCounts      []int
	userErr         error
}

func (s *userRPMCacheStub) IncrementUserGroupRPM(_ context.Context, _, _ int64) (int, error) {
	idx := int(atomic.AddInt32(&s.userGroupCalls, 1)) - 1
	if s.userGroupErr != nil {
		return 0, s.userGroupErr
	}
	if idx < len(s.userGroupCounts) {
		return s.userGroupCounts[idx], nil
	}
	return 1, nil
}

func (s *userRPMCacheStub) IncrementUserRPM(_ context.Context, _ int64) (int, error) {
	idx := int(atomic.AddInt32(&s.userCalls, 1)) - 1
	if s.userErr != nil {
		return 0, s.userErr
	}
	if idx < len(s.userCounts) {
		return s.userCounts[idx], nil
	}
	return 1, nil
}

func (s *userRPMCacheStub) GetUserGroupRPM(_ context.Context, _, _ int64) (int, error) {
	return 0, nil
}

func (s *userRPMCacheStub) GetUserRPM(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

// rpmOverrideRepoStub 专用于 checkRPM 分支测试，只实现必要方法。

func newBillingServiceForRPM(t *testing.T, cache UserRPMCache) *BillingCacheService {
	t.Helper()
	// 用 nil BillingCache 走 "无缓存" 分支，避免 CheckBillingEligibility 副作用。
	// 我们只直接测 checkRPM。
	svc := NewBillingCacheService(nil, nil, nil, nil, cache, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	return svc
}

func TestBillingCacheService_CheckRPM_UserLimitIsGlobalHardCap(t *testing.T) {
	// 分组计数默认返回 1，用户计数依次返回 1、2、3。
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3}}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 2}
	group := &Group{ID: 10, RPMLimit: 100}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrUserRPMExceeded, "user 全局上限生效")
}

func TestBillingCacheService_CheckRPM_GroupLimit(t *testing.T) {
	// user-group 计数: 5, 6；user 计数: 默认 1（不干扰）
	cache := &userRPMCacheStub{userGroupCounts: []int{5, 6}}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 999} // 全局上限很高，group 先超
	group := &Group{ID: 10, RPMLimit: 5}

	require.NoError(t, svc.checkRPM(context.Background(), user, group))                      // ug=5, user=1, 都没超
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrGroupRPMExceeded) // ug=6 > 5

	require.EqualValues(t, 2, atomic.LoadInt32(&cache.userGroupCalls))
	// 并行模式：第 1 次 group 没超 → 继续检查 user；第 2 次 group 超了 → 直接 return，不检查 user
	require.EqualValues(t, 1, atomic.LoadInt32(&cache.userCalls), "group 未超时 user 也应检查；group 超时直接返回")
}

func TestBillingCacheService_CheckRPM_UserLevelFallbackWhenGroupUnlimited(t *testing.T) {
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3}}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 2}
	group := &Group{ID: 10, RPMLimit: 0} // 分组未设限

	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, group), ErrUserRPMExceeded)

	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls), "group 未设限时不应 INCR user-group 键")
	require.EqualValues(t, 3, atomic.LoadInt32(&cache.userCalls))
}

func TestBillingCacheService_CheckRPM_NoLimitsConfiguredIsNoop(t *testing.T) {
	cache := &userRPMCacheStub{}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 0}
	group := &Group{ID: 10, RPMLimit: 0}

	for i := 0; i < 10; i++ {
		require.NoError(t, svc.checkRPM(context.Background(), user, group))
	}
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls))
}

func TestBillingCacheService_CheckRPM_RedisErrorFailOpen(t *testing.T) {
	cache := &userRPMCacheStub{userGroupErr: errors.New("redis unavailable")}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 0}
	group := &Group{ID: 10, RPMLimit: 5}

	// Redis 故障时应 fail-open，不拒绝请求
	require.NoError(t, svc.checkRPM(context.Background(), user, group))
	require.EqualValues(t, 1, atomic.LoadInt32(&cache.userGroupCalls))
}

func TestBillingCacheService_CheckRPM_NoGroupUsesUserOnly(t *testing.T) {
	cache := &userRPMCacheStub{userCounts: []int{1, 2, 3}}
	svc := newBillingServiceForRPM(t, cache)

	user := &User{ID: 1, RPMLimit: 2}

	// 无分组时执行用户级限流。
	require.NoError(t, svc.checkRPM(context.Background(), user, nil))
	require.NoError(t, svc.checkRPM(context.Background(), user, nil))
	require.ErrorIs(t, svc.checkRPM(context.Background(), user, nil), ErrUserRPMExceeded)

	require.EqualValues(t, 3, atomic.LoadInt32(&cache.userCalls))
}

func TestBillingCacheService_CheckRPM_NilUserIsNoop(t *testing.T) {
	cache := &userRPMCacheStub{}
	svc := newBillingServiceForRPM(t, cache)

	require.NoError(t, svc.checkRPM(context.Background(), nil, &Group{ID: 1, RPMLimit: 10}))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userGroupCalls))
	require.EqualValues(t, 0, atomic.LoadInt32(&cache.userCalls))
}

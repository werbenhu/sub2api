package admin

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type subscriptionPlanServiceStub struct {
	plan *dbent.SubscriptionPlan
}

func (s subscriptionPlanServiceStub) ListPlans(context.Context) ([]*dbent.SubscriptionPlan, error) {
	return nil, nil
}
func (s subscriptionPlanServiceStub) CreatePlan(context.Context, service.CreatePlanRequest) (*dbent.SubscriptionPlan, error) {
	panic("unexpected CreatePlan call")
}
func (s subscriptionPlanServiceStub) UpdatePlan(context.Context, int64, service.UpdatePlanRequest) (*dbent.SubscriptionPlan, error) {
	panic("unexpected UpdatePlan call")
}
func (s subscriptionPlanServiceStub) DeletePlan(context.Context, int64) error {
	panic("unexpected DeletePlan call")
}
func (s subscriptionPlanServiceStub) GetPlan(context.Context, int64) (*dbent.SubscriptionPlan, error) {
	return s.plan, nil
}

type subscriptionPlanGroupRepoStub struct {
	service.GroupRepository
}

func (subscriptionPlanGroupRepoStub) GetByID(context.Context, int64) (*service.Group, error) {
	return &service.Group{ID: 91, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeSubscription}, nil
}

type subscriptionPlanUserRepoStub struct {
	service.UserSubscriptionRepository
	created *service.UserSubscription
}

func (s *subscriptionPlanUserRepoStub) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (s *subscriptionPlanUserRepoStub) Create(_ context.Context, sub *service.UserSubscription) error {
	sub.ID = 100
	s.created = sub
	return nil
}
func (s *subscriptionPlanUserRepoStub) GetByID(_ context.Context, _ int64) (*service.UserSubscription, error) {
	return s.created, nil
}

func TestAssignPlanUsesServerSidePlanConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &subscriptionPlanUserRepoStub{}
	subscriptionService := service.NewSubscriptionService(subscriptionPlanGroupRepoStub{}, repo, nil, nil, nil)
	handler := NewSubscriptionHandler(subscriptionService, subscriptionPlanServiceStub{plan: &dbent.SubscriptionPlan{
		ID: 9, GroupID: 91, ValidityDays: 90,
		Budget: &domain.SubscriptionBudget{AmountUSD: 20, ResetPeriod: "hours", ResetHours: 6},
	}})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/subscriptions/assign-plan", strings.NewReader(`{"user_id":42,"plan_id":9}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.AssignPlan(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, repo.created)
	require.Equal(t, int64(42), repo.created.UserID)
	require.Equal(t, int64(91), repo.created.GroupID)
	require.NotNil(t, repo.created.Budget)
	require.Equal(t, 20.0, repo.created.Budget.AmountUSD)
	require.Equal(t, 6, repo.created.Budget.ResetHours)
	require.WithinDuration(t, time.Now().AddDate(0, 0, 90), repo.created.ExpiresAt, 5*time.Second)
}

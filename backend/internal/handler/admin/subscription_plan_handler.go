package admin

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type subscriptionPlanService interface {
	ListPlans(context.Context) ([]*dbent.SubscriptionPlan, error)
	CreatePlan(context.Context, service.CreatePlanRequest) (*dbent.SubscriptionPlan, error)
	UpdatePlan(context.Context, int64, service.UpdatePlanRequest) (*dbent.SubscriptionPlan, error)
	DeletePlan(context.Context, int64) error
	GetPlan(context.Context, int64) (*dbent.SubscriptionPlan, error)
}

type SubscriptionPlanRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	GroupID      int64  `json:"group_id" binding:"required"`
	ValidityDays int    `json:"validity_days" binding:"required,min=1,max=36500"`
	SortOrder    int    `json:"sort_order"`
}

type AssignSubscriptionPlanRequest struct {
	UserID int64  `json:"user_id" binding:"required"`
	PlanID int64  `json:"plan_id" binding:"required"`
	Notes  string `json:"notes"`
}

type BulkAssignSubscriptionPlanRequest struct {
	UserIDs []int64 `json:"user_ids" binding:"required,min=1,max=100,dive,gt=0"`
	PlanID  int64   `json:"plan_id" binding:"required"`
	Notes   string  `json:"notes"`
}

func (h *SubscriptionHandler) ListPlans(c *gin.Context) {
	if h.planService == nil {
		response.InternalError(c, "Subscription plan service is not configured")
		return
	}
	plans, err := h.planService.ListPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}

func (h *SubscriptionHandler) CreatePlan(c *gin.Context) {
	var req SubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.planService.CreatePlan(c.Request.Context(), service.CreatePlanRequest{
		GroupID: req.GroupID, Name: req.Name, Description: req.Description,
		ValidityDays: req.ValidityDays, ValidityUnit: "day", SortOrder: req.SortOrder,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plan)
}

func (h *SubscriptionHandler) UpdatePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req SubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	validityUnit := "day"
	plan, err := h.planService.UpdatePlan(c.Request.Context(), id, service.UpdatePlanRequest{
		GroupID: &req.GroupID, Name: &req.Name, Description: &req.Description,
		ValidityDays: &req.ValidityDays, ValidityUnit: &validityUnit, SortOrder: &req.SortOrder,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plan)
}

func (h *SubscriptionHandler) DeletePlan(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.planService.DeletePlan(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "subscription plan deleted"})
}

func (h *SubscriptionHandler) AssignPlan(c *gin.Context) {
	var req AssignSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.planService.GetPlan(c.Request.Context(), req.PlanID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subscription, err := h.subscriptionService.AssignSubscription(c.Request.Context(), &service.AssignSubscriptionInput{
		UserID: req.UserID, GroupID: plan.GroupID, ValidityDays: plan.ValidityDays,
		AssignedBy: getAdminIDFromContext(c), Notes: req.Notes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, subscription)
}

func (h *SubscriptionHandler) BulkAssignPlan(c *gin.Context) {
	var req BulkAssignSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	plan, err := h.planService.GetPlan(c.Request.Context(), req.PlanID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.subscriptionService.BulkAssignSubscription(c.Request.Context(), &service.BulkAssignSubscriptionInput{
		UserIDs: req.UserIDs, GroupID: plan.GroupID, ValidityDays: plan.ValidityDays,
		AssignedBy: getAdminIDFromContext(c), Notes: req.Notes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

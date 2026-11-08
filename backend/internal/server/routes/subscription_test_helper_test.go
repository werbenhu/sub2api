package routes

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type activeSubscriptionRepoStub struct {
	service.UserSubscriptionRepository
}

func (activeSubscriptionRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	return &service.UserSubscription{
		UserID:    userID,
		GroupID:   groupID,
		Status:    service.SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

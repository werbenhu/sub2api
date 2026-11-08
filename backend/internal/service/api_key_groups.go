package service

import (
	"context"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func normalizeAPIKeyGroupIDs(groupIDs []int64) []int64 {
	out := make([]int64, 0, len(groupIDs))
	seen := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		out = append(out, groupID)
	}
	return out
}

func (s *APIKeyService) validateAPIKeyGroups(ctx context.Context, user *User, groupIDs []int64) error {
	for _, groupID := range groupIDs {
		group, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}
		if group.Status != StatusActive || !s.canUserBindGroup(ctx, user, group) {
			return ErrGroupNotAllowed
		}
	}
	return nil
}

func (s *APIKeyService) defaultUserGroupIDs(ctx context.Context, user *User) []int64 {
	if s == nil || user == nil || s.userSubRepo == nil || s.groupRepo == nil {
		return nil
	}
	subscriptions, err := s.userSubRepo.ListActiveByUserID(ctx, user.ID)
	if err != nil {
		return nil
	}
	groupIDs := make([]int64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		group, err := s.groupRepo.GetByID(ctx, subscription.GroupID)
		if err == nil && group != nil && group.Status == StatusActive {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	return normalizeAPIKeyGroupIDs(groupIDs)
}

func (s *APIKeyService) ResolveGroupForModel(ctx context.Context, apiKey *APIKey, model, endpoint string) error {
	if s == nil || apiKey == nil || !apiKey.HasMultipleGroups() || strings.TrimSpace(model) == "" {
		return nil
	}
	groupIDs := apiKey.ConfiguredGroupIDs()
	var fallback *Group
	for _, groupID := range groupIDs {
		group, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil || group == nil || group.Status != StatusActive {
			continue
		}
		if s.userSubRepo != nil {
			if _, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, apiKey.UserID, groupID); err != nil {
				continue
			}
		}
		if fallback == nil {
			fallback = group
		}
		if s.compositeResolver == nil {
			continue
		}
		decision, err := s.compositeResolver.ResolvePreview(ctx, groupID, model, endpoint)
		if err != nil {
			return err
		}
		if decision.OAuthAmbiguous() {
			return infraerrors.BadRequest("COMPOSITE_ROUTE_CONFIG_INVALID", decision.AmbiguousMessage())
		}
		if decision.Matched && decision.Source != CompositeRouteSourceDetector {
			apiKey.GroupID = int64Pointer(groupID)
			apiKey.Group = group
			return nil
		}
	}
	if fallback != nil {
		apiKey.GroupID = int64Pointer(fallback.ID)
		apiKey.Group = fallback
	}
	return nil
}

func (s *APIKeyService) ActiveSubscriptionForKey(ctx context.Context, apiKey *APIKey) (*UserSubscription, error) {
	if s == nil || s.userSubRepo == nil || apiKey == nil || apiKey.GroupID == nil {
		return nil, ErrSubscriptionNotFound
	}
	return s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, apiKey.UserID, *apiKey.GroupID)
}

func int64Pointer(value int64) *int64 {
	return &value
}

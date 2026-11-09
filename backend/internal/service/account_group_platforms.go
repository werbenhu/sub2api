package service

import (
	"context"
	"errors"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// SupportsAccount preserves mixed scheduling for legacy single-platform groups.
// Composite selections always constrain the account's concrete platform.
func (g *Group) SupportsAccount(account *Account) bool {
	if g == nil || account == nil {
		return false
	}
	return g.SupportsPlatform(account.Platform) ||
		(g.Platform != PlatformComposite && account.IsMixedSchedulingEnabled() &&
			(g.Platform == PlatformAnthropic || g.Platform == PlatformGemini))
}

func validateAccountGroupPlatforms(ctx context.Context, repo GroupRepository, account *Account, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	if repo == nil {
		return errors.New("group repository not configured")
	}
	seen := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		group, err := repo.GetByIDLite(ctx, groupID)
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}
		if !group.SupportsAccount(account) {
			return infraerrors.BadRequest("GROUP_PLATFORM_ACCOUNT_MISMATCH",
				fmt.Sprintf("account platform %s is not selected in group %s", account.Platform, group.Name))
		}
	}
	return nil
}

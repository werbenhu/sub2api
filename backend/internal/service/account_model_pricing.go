package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const accountModelPricingKey = "model_pricing"

type accountPricingContextKey struct{}
type accountPricingContext struct {
	platform string
	entries  []ChannelModelPricing
}

// Only the selected account's billing context may supply account prices.
func withAccountModelPricing(ctx context.Context, account *Account) context.Context {
	value := accountPricingContext{}
	if account != nil {
		value.platform = account.Platform
		value.entries, _ = accountModelPricing(account.Extra)
	}
	return context.WithValue(ctx, accountPricingContextKey{}, value)
}

func accountModelPricing(extra map[string]any) ([]ChannelModelPricing, error) {
	value, ok := extra[accountModelPricingKey]
	if !ok || value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var entries []ChannelModelPricing
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func validateAccountModelPricing(extra map[string]any) error {
	entries, err := accountModelPricing(extra)
	if err == nil {
		err = validatePricingEntries(entries)
	}
	if err == nil {
		for _, entry := range entries {
			if len(entry.Models) == 0 {
				err = fmt.Errorf("model pricing requires at least one model")
				break
			}
			for _, price := range []*float64{entry.InputPrice, entry.OutputPrice, entry.CacheWritePrice, entry.CacheWrite1hPrice, entry.CacheReadPrice, entry.ImageInputPrice, entry.ImageOutputPrice, entry.PerRequestPrice} {
				if price != nil && (math.IsNaN(*price) || math.IsInf(*price, 0) || *price < 0) {
					err = fmt.Errorf("model prices must be finite and non-negative")
					break
				}
			}
		}
	}
	if err != nil {
		return infraerrors.BadRequest("ACCOUNT_MODEL_PRICING_INVALID", err.Error())
	}
	return nil
}

func matchAccountModelPricing(ctx context.Context, model string) *ChannelModelPricing {
	value, _ := ctx.Value(accountPricingContextKey{}).(accountPricingContext)
	return matchGroupModelPricing(&Group{ModelPricing: value.entries}, value.platform, model)
}

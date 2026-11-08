package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const compositeRouteCursorKeyPrefix = "composite_route:cursor:"

type compositeRouteCursorCache struct {
	rdb *redis.Client
}

func NewCompositeRouteCursorCache(rdb *redis.Client) service.CompositeRouteCursorCache {
	return &compositeRouteCursorCache{rdb: rdb}
}

func (c *compositeRouteCursorCache) NextCompositeRouteCursor(ctx context.Context, key string, ttl time.Duration) (uint64, error) {
	redisKey := compositeRouteCursorKeyPrefix + key
	pipe := c.rdb.TxPipeline()
	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return uint64(incr.Val() - 1), nil
}

func (c *compositeRouteCursorCache) PeekCompositeRouteCursor(ctx context.Context, key string) (uint64, error) {
	value, err := c.rdb.Get(ctx, compositeRouteCursorKeyPrefix+key).Uint64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return value, err
}

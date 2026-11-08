package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCompositeRouteCursorCacheIncrementsAndPeeks(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache := NewCompositeRouteCursorCache(client)
	ctx := context.Background()

	index, err := cache.PeekCompositeRouteCursor(ctx, "group|model")
	require.NoError(t, err)
	require.Zero(t, index)
	for _, expected := range []uint64{0, 1, 2} {
		index, err = cache.NextCompositeRouteCursor(ctx, "group|model", time.Hour)
		require.NoError(t, err)
		require.Equal(t, expected, index)
	}
	index, err = cache.PeekCompositeRouteCursor(ctx, "group|model")
	require.NoError(t, err)
	require.Equal(t, uint64(3), index)
	require.Equal(t, time.Hour, server.TTL(compositeRouteCursorKeyPrefix+"group|model"))
}

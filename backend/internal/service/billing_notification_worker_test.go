package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsageNotificationsBoundWorkAndCancelOnStop(t *testing.T) {
	w := newUsageNotificationWorker()
	t.Cleanup(w.stop)
	started := make(chan context.Context, usageNotificationWorkers)
	var running, queuedRan atomic.Int64
	for range usageNotificationWorkers {
		require.True(t, w.submit(func(ctx context.Context) {
			running.Add(1)
			defer running.Add(-1)
			started <- ctx
			<-ctx.Done()
		}))
	}
	for range usageNotificationWorkers {
		select {
		case ctx := <-started:
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			require.InDelta(t, usageNotificationTimeout.Seconds(), time.Until(deadline).Seconds(), 2)
		case <-time.After(2 * time.Second):
			t.Fatal("notification worker did not start")
		}
	}
	for range usageNotificationQueueSize {
		require.True(t, w.submit(func(context.Context) { queuedRan.Add(1) }))
	}
	require.False(t, w.submit(func(context.Context) { queuedRan.Add(1) }))
	require.Equal(t, int64(usageNotificationWorkers), running.Load())
	require.Equal(t, uint64(1), w.dropped.Load())

	w.stop()
	w.stop()
	require.Zero(t, running.Load())
	require.Zero(t, queuedRan.Load(), "shutdown must not start queued settings reads or email deliveries")
	require.False(t, w.submit(func(context.Context) { t.Error("stopped worker accepted work") }))
}

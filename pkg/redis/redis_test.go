package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	"digital.vasic.ratelimiter/pkg/limiter"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMiniredis(t *testing.T) (*miniredis.Miniredis, goredis.Cmdable) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
		DialTimeout: 30 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, PoolTimeout: 30 * time.Second,
	})
	t.Cleanup(func() { client.Close() })

	return mr, client
}

func TestAllowWithinLimit(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   5,
		Window: time.Second,
		Burst:  5,
	}

	rl := New(client, cfg)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		result, err := rl.Allow(ctx, "test-key")
		require.NoError(t, err)
		assert.True(t, result.Allowed, "request %d should be allowed", i+1)
		assert.Equal(t, 5, result.Limit)
		assert.Equal(t, 5-i-1, result.Remaining, "remaining should decrease")
	}
}

func TestAllowExceedingLimit(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   3,
		Window: time.Second,
		Burst:  3,
	}

	rl := New(client, cfg)
	ctx := context.Background()

	// Exhaust the limit
	for i := 0; i < 3; i++ {
		result, err := rl.Allow(ctx, "test-key")
		require.NoError(t, err)
		assert.True(t, result.Allowed)
	}

	// Next request should be denied
	result, err := rl.Allow(ctx, "test-key")
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, 0, result.Remaining)
}

func TestSeparateKeys(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   2,
		Window: time.Second,
		Burst:  2,
	}

	rl := New(client, cfg)
	ctx := context.Background()

	// Exhaust limit for key-a
	rl.Allow(ctx, "key-a")
	rl.Allow(ctx, "key-a")
	result, _ := rl.Allow(ctx, "key-a")
	assert.False(t, result.Allowed)

	// key-b should still have capacity
	result, err := rl.Allow(ctx, "key-b")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 1, result.Remaining)
}

func TestReset(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   2,
		Window: time.Second,
		Burst:  2,
	}

	rl := New(client, cfg)
	ctx := context.Background()

	// Exhaust limit
	rl.Allow(ctx, "key")
	rl.Allow(ctx, "key")
	result, _ := rl.Allow(ctx, "key")
	assert.False(t, result.Allowed)

	// Reset the key
	err := rl.Reset(ctx, "key")
	require.NoError(t, err)

	// Should be allowed again
	result, err = rl.Allow(ctx, "key")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, 1, result.Remaining)
}

func TestWithPrefix(t *testing.T) {
	mr, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   5,
		Window: time.Second,
		Burst:  5,
	}

	rl := New(client, cfg, WithPrefix("myapp:rl:"))
	ctx := context.Background()

	rl.Allow(ctx, "user123")

	// Verify the key in Redis has the correct prefix
	keys := mr.Keys()
	require.Len(t, keys, 1)
	assert.Equal(t, "myapp:rl:user123", keys[0])
}

func TestWindowExpiry(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   2,
		Window: 100 * time.Millisecond,
		Burst:  2,
	}

	rl := New(client, cfg)
	ctx := context.Background()

	// Exhaust limit
	rl.Allow(ctx, "key")
	rl.Allow(ctx, "key")
	result, _ := rl.Allow(ctx, "key")
	assert.False(t, result.Allowed)

	// Sleep well past the window so that the Lua script's ZREMRANGEBYSCORE
	// will remove the old entries based on the real wall-clock time.
	time.Sleep(250 * time.Millisecond)

	// Should be allowed again since old entries are outside the window
	result, err := rl.Allow(ctx, "key")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestDefaultBurstEqualsRate(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   10,
		Window: time.Second,
		Burst:  0, // should default to Rate
	}

	rl := New(client, cfg)
	ctx := context.Background()

	result, err := rl.Allow(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, 10, result.Limit)
}

func TestConcurrentAccess(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   1000,
		Window: 5 * time.Second,
		Burst:  1000,
	}

	rl := New(client, cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	allowedCount := 0
	var mu sync.Mutex

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				result, err := rl.Allow(ctx, "concurrent-key")
				if err == nil && result.Allowed {
					mu.Lock()
					allowedCount++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	// All 500 requests should be allowed (within 1000 limit)
	assert.Equal(t, 500, allowedCount)
}

func TestRetryAfterOnDenied(t *testing.T) {
	_, client := setupMiniredis(t)

	cfg := &limiter.Config{
		Rate:   1,
		Window: time.Second,
		Burst:  1,
	}

	rl := New(client, cfg)
	ctx := context.Background()

	// First request allowed
	result, _ := rl.Allow(ctx, "key")
	assert.True(t, result.Allowed)

	// Second request denied with retry-after
	result, _ = rl.Allow(ctx, "key")
	assert.False(t, result.Allowed)
	assert.True(t, result.RetryAfter >= 0, "retry after should be non-negative")
}

// Verify that the RateLimiter satisfies the Limiter interface
var _ limiter.Limiter = (*RateLimiter)(nil)

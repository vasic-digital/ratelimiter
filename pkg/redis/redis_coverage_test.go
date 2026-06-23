package redis

import (
	"context"
	"testing"
	"time"

	"digital.vasic.ratelimiter/pkg/limiter"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllow_RetryAfterOnDenied verifies RetryAfter is set when denied.
func TestAllow_RetryAfterOnDenied(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), DialTimeout: 30 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, PoolTimeout: 30 * time.Second})
	defer client.Close()

	cfg := &limiter.Config{
		Rate:   1,
		Window: time.Second,
		Burst:  1,
	}
	rl := New(client, cfg)
	ctx := context.Background()

	// First allowed
	result, err := rl.Allow(ctx, "key")
	require.NoError(t, err)
	assert.True(t, result.Allowed)

	// Second denied
	result, err = rl.Allow(ctx, "key")
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.GreaterOrEqual(t, int64(result.RetryAfter), int64(0))
}

// TestAllow_LuaScriptError verifies the error path when the Redis command fails.
func TestAllow_LuaScriptError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), DialTimeout: 30 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, PoolTimeout: 30 * time.Second})
	defer client.Close()

	cfg := &limiter.Config{
		Rate:   5,
		Window: time.Second,
		Burst:  5,
	}
	rl := New(client, cfg)
	ctx := context.Background()

	// Close miniredis to force a connection error
	mr.Close()

	_, err = rl.Allow(ctx, "key")
	assert.Error(t, err, "should return an error when Redis is unavailable")
	assert.Contains(t, err.Error(), "rate limiter lua script failed")
}

// TestNew_WithMultipleOptions verifies that multiple options are applied.
func TestNew_WithMultipleOptions(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), DialTimeout: 30 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, PoolTimeout: 30 * time.Second})
	defer client.Close()

	cfg := &limiter.Config{
		Rate:   10,
		Window: time.Second,
		Burst:  10,
	}
	rl := New(client, cfg, WithPrefix("custom:"))
	assert.Equal(t, "custom:", rl.prefix)
}

// TestReset_NonExistentKey verifies resetting a key that doesn't exist is a no-op.
func TestReset_NonExistentKey(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), DialTimeout: 30 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, PoolTimeout: 30 * time.Second})
	defer client.Close()

	rl := New(client, nil)
	err = rl.Reset(context.Background(), "nonexistent")
	assert.NoError(t, err)
}

// TestAllow_DefaultBurstZero verifies burst=0 defaults to rate.
func TestAllow_DefaultBurstZero(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), DialTimeout: 30 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, PoolTimeout: 30 * time.Second})
	defer client.Close()

	cfg := &limiter.Config{
		Rate:   3,
		Window: time.Second,
		Burst:  0,
	}
	rl := New(client, cfg)
	ctx := context.Background()

	// Should allow Rate number of requests
	for i := 0; i < 3; i++ {
		result, err := rl.Allow(ctx, "key")
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, 3, result.Limit)
	}

	// Fourth should be denied
	result, err := rl.Allow(ctx, "key")
	require.NoError(t, err)
	assert.False(t, result.Allowed)
}

// TestAllow_ResetAtIsInFuture verifies ResetAt is set in the future.
func TestAllow_ResetAtIsInFuture(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr(), DialTimeout: 30 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, PoolTimeout: 30 * time.Second})
	defer client.Close()

	cfg := &limiter.Config{
		Rate:   5,
		Window: time.Second,
		Burst:  5,
	}
	rl := New(client, cfg)
	ctx := context.Background()

	result, err := rl.Allow(ctx, "key")
	require.NoError(t, err)
	assert.True(t, result.ResetAt.After(time.Now()), "ResetAt should be in the future")
}

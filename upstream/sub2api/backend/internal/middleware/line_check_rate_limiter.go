package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis time and atomic admission keep the rolling window shared across instances.
var slidingWindowScript = redis.NewScript(`
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)
local count = redis.call('ZCARD', KEYS[1])
if count >= limit then
  local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
  return {0, count, math.max(1, tonumber(oldest[2]) + window - now)}
end
local member = clock[1] .. ':' .. clock[2] .. ':' .. count
redis.call('ZADD', KEYS[1], now, member)
redis.call('PEXPIRE', KEYS[1], window)
return {1, count + 1, 0}
`)

// AllowSlidingWindow counts admitted requests in the preceding window. Rejected
// requests do not extend the window or consume additional allowance.
func (r *RateLimiter) AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration) (AllowResult, error) {
	if r == nil || r.redis == nil || limit < 1 || window <= 0 {
		return AllowResult{}, fmt.Errorf("invalid sliding window limiter configuration")
	}
	values, err := slidingWindowScript.Run(ctx, r.redis, []string{r.prefix + key}, windowTTLMillis(window), limit).Slice()
	if err != nil {
		return AllowResult{}, err
	}
	if len(values) != 3 {
		return AllowResult{}, fmt.Errorf("sliding window script returned %d values", len(values))
	}
	allowed, err := parseInt64(values[0])
	if err != nil {
		return AllowResult{}, err
	}
	count, err := parseInt64(values[1])
	if err != nil {
		return AllowResult{}, err
	}
	retryMillis, err := parseInt64(values[2])
	if err != nil {
		return AllowResult{}, err
	}
	return AllowResult{Allowed: allowed == 1, Count: count, RetryAfter: time.Duration(retryMillis) * time.Millisecond}, nil
}

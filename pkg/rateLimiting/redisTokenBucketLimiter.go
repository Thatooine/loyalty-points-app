package rateLimiting

import (
	"context"
	"time"

	"github.com/mennanov/limiters"
)

// RedisTokenBucketRateLimiter extends TokenBucketRateLimiter with Redis-backed
// state management. The Redis backend is what makes the limit shared across all
// app instances rather than per-process.
type RedisTokenBucketRateLimiter interface {
	TokenBucketRateLimiter
	TokenStateBackend(ctx context.Context, key string, ttl time.Duration) (*TokenStateBackendResponse, error)
}

type TokenStateBackendResponse struct {
	StateBackend *limiters.TokenBucketRedis
}

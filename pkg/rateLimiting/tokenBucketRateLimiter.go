package rateLimiting

import (
	"context"
	"time"

	"github.com/mennanov/limiters"
)

// TokenBucketRateLimiter defines the rate limiting operations the middlewares
// depend on. Keeping it an interface lets the data layer (Redis) be swapped or
// mocked without touching transport.
type TokenBucketRateLimiter interface {
	TokenBucket(ctx context.Context, request TokenBucketRequest, stateBackend limiters.TokenBucketStateBackend) *TokenBucketResponse
	Limit(ctx context.Context, tokenBucket *limiters.TokenBucket) (*LimitResponse, error)
}

type TokenBucketRequest struct {
	RefillRate time.Duration
	Capacity   int64
}

type TokenBucketResponse struct {
	TokenBucket *limiters.TokenBucket
}

type LimitResponse struct {
	TimeToRetry time.Duration
}

package rateLimiting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/mennanov/limiters"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	pkgRateLimiting "github.com/Thatooine/loyalty-points-app/pkg/rateLimiting"
)

type RedisRateLimiterImpl struct {
	redisClient *redis.Client
	locker      limiters.DistLocker
}

func NewRedisRateLimiterImpl(redisClient *redis.Client) *RedisRateLimiterImpl {
	pool := goredis.NewPool(redisClient)
	return &RedisRateLimiterImpl{
		redisClient: redisClient,
		// A distributed lock makes the read-modify-write on a bucket safe across
		// concurrent requests and app instances sharing the same Redis.
		locker: limiters.NewLockRedis(pool, "rate_limiter_lock"),
	}
}

func (r *RedisRateLimiterImpl) TokenStateBackend(_ context.Context, key string, ttl time.Duration) (*pkgRateLimiting.TokenStateBackendResponse, error) {
	stateBackend := limiters.NewTokenBucketRedis(r.redisClient, key, ttl, false)
	return &pkgRateLimiting.TokenStateBackendResponse{
		StateBackend: stateBackend,
	}, nil
}

func (r *RedisRateLimiterImpl) TokenBucket(_ context.Context, request pkgRateLimiting.TokenBucketRequest, stateBackend limiters.TokenBucketStateBackend) *pkgRateLimiting.TokenBucketResponse {
	tokenBucket := limiters.NewTokenBucket(
		request.Capacity,
		request.RefillRate,
		r.locker,
		stateBackend,
		limiters.NewSystemClock(),
		nil,
	)

	return &pkgRateLimiting.TokenBucketResponse{
		TokenBucket: tokenBucket,
	}
}

func (r *RedisRateLimiterImpl) Limit(ctx context.Context, tokenBucket *limiters.TokenBucket) (*pkgRateLimiting.LimitResponse, error) {
	wait, err := tokenBucket.Limit(ctx)
	if err != nil {
		if errors.Is(err, limiters.ErrLimitExhausted) {
			return nil, limiters.ErrLimitExhausted
		}
		log.Ctx(ctx).Error().Err(err).Msg("rate limiting failed")
		return nil, fmt.Errorf("rate limiting failed: %w", err)
	}

	return &pkgRateLimiting.LimitResponse{
		TimeToRetry: wait,
	}, nil
}

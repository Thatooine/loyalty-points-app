package rateLimiting

import (
	"errors"
	"net/http"
	"time"

	"github.com/mennanov/limiters"
	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
)

type userRateLimiterMiddleware struct {
	next        http.Handler
	rateLimiter RedisTokenBucketRateLimiter
	refillRate  time.Duration
	capacity    int64
	ttl         time.Duration
}

// NewUserRateLimiterMiddleware returns a gorilla/mux-compatible middleware that
// rate limits authenticated requests by UserID using a Redis-backed token
// bucket. It must be mounted after the authorization middleware so the login
// claim is on the context. Requests with no claim (public methods) pass through
// untouched — the IP limiter covers those.
func NewUserRateLimiterMiddleware(
	rateLimiter RedisTokenBucketRateLimiter,
	capacity int64,
	refillRate time.Duration,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &userRateLimiterMiddleware{
			next:        next,
			rateLimiter: rateLimiter,
			refillRate:  refillRate,
			capacity:    capacity,
			ttl:         time.Duration(int64(refillRate) * capacity),
		}
	}
}

func (m *userRateLimiterMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		m.next.ServeHTTP(w, r)
		return
	}

	key := "user_token_bucket:" + claim.UserID

	stateBackend, err := m.rateLimiter.TokenStateBackend(ctx, key, m.ttl)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("rate limit: failed to create token state backend")
		jsonrpc.WriteErrorStatus(w, http.StatusInternalServerError, nil, jsonrpc.CodeInternal, "internal server error", "internal")
		return
	}

	tokenBucket := m.rateLimiter.TokenBucket(ctx, TokenBucketRequest{
		RefillRate: m.refillRate,
		Capacity:   m.capacity,
	}, stateBackend.StateBackend)

	if _, err := m.rateLimiter.Limit(ctx, tokenBucket.TokenBucket); err != nil {
		if errors.Is(err, limiters.ErrLimitExhausted) {
			log.Ctx(ctx).Warn().Str("userID", claim.UserID).Msg("rate limit: user limit exceeded")
			jsonrpc.WriteErrorStatus(w, http.StatusTooManyRequests, nil, jsonrpc.CodeTooManyRequests, "too many requests", "rate_limited")
			return
		}
		log.Ctx(ctx).Error().Err(err).Msg("rate limit: user rate limiting error")
		jsonrpc.WriteErrorStatus(w, http.StatusInternalServerError, nil, jsonrpc.CodeInternal, "internal server error", "internal")
		return
	}

	m.next.ServeHTTP(w, r)
}

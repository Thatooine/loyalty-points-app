package rateLimiting

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/mennanov/limiters"
	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
)

// maxPeekBodyBytes caps the body this middleware buffers to read the JSON-RPC
// method. It runs before authentication, so the read must be bounded or an
// unauthenticated caller could stream an arbitrarily large body through
// io.ReadAll. It mirrors the cap in the authorization middleware.
const maxPeekBodyBytes = 4 << 20 // 4 MiB

type ipRateLimiterMiddleware struct {
	next        http.Handler
	rateLimiter RedisTokenBucketRateLimiter
	methods     map[string]bool
	refillRate  time.Duration
	capacity    int64
	ttl         time.Duration
}

// NewIPRateLimiterMiddleware returns a gorilla/mux-compatible middleware that
// rate limits requests by client IP using a Redis-backed token bucket.
//
// Unlike a REST router where each route is its own handler, every JSON-RPC
// method shares the single /api endpoint, so the middleware peeks the method
// out of the request body and only consumes a token for methods in `methods`.
// Wire it with the public auth methods (login/register) so credential
// brute-force is throttled per IP without limiting ordinary authenticated
// traffic (which the user limiter covers).
func NewIPRateLimiterMiddleware(
	rateLimiter RedisTokenBucketRateLimiter,
	methods map[string]bool,
	capacity int64,
	refillRate time.Duration,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &ipRateLimiterMiddleware{
			next:        next,
			rateLimiter: rateLimiter,
			methods:     methods,
			refillRate:  refillRate,
			capacity:    capacity,
			ttl:         time.Duration(int64(refillRate) * capacity),
		}
	}
}

func (m *ipRateLimiterMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	method, ok := m.peekMethod(w, r)
	if !ok {
		// Body was unreadable/oversize; the authorization middleware downstream
		// produces the canonical error envelope. We just decline to rate limit a
		// request we can't classify and let it proceed to that rejection.
		m.next.ServeHTTP(w, r)
		return
	}

	// Only the targeted (public auth) methods consume a token; everything else
	// passes straight through to the user limiter / dispatch.
	if !m.methods[method] {
		m.next.ServeHTTP(w, r)
		return
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	key := "ip_token_bucket:" + ip

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
			log.Ctx(ctx).Warn().Str("ip", ip).Str("method", method).Msg("rate limit: ip limit exceeded")
			jsonrpc.WriteErrorStatus(w, http.StatusTooManyRequests, nil, jsonrpc.CodeTooManyRequests, "too many requests", "rate_limited")
			return
		}
		log.Ctx(ctx).Error().Err(err).Msg("rate limit: ip rate limiting error")
		jsonrpc.WriteErrorStatus(w, http.StatusInternalServerError, nil, jsonrpc.CodeInternal, "internal server error", "internal")
		return
	}

	m.next.ServeHTTP(w, r)
}

// peekMethod buffers the (capped) body, reads the JSON-RPC method, and restores
// the body so downstream handlers can read it again. ok is false when the body
// could not be read or parsed.
func (m *ipRateLimiterMiddleware) peekMethod(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPeekBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var envelope jsonrpc.RequestEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Method == "" {
		return "", false
	}
	return envelope.Method, true
}

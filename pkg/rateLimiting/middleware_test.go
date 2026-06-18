package rateLimiting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mennanov/limiters"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
)

// mockLimiter implements RedisTokenBucketRateLimiter without any Redis. The
// bucket value it returns is unused by the mock's Limit, which just replays a
// configured outcome — enough to drive the middleware's branches.
type mockLimiter struct {
	limitErr   error
	stateErr   error
	limitCalls int
}

func (m *mockLimiter) TokenStateBackend(_ context.Context, _ string, _ time.Duration) (*TokenStateBackendResponse, error) {
	if m.stateErr != nil {
		return nil, m.stateErr
	}
	return &TokenStateBackendResponse{}, nil
}

func (m *mockLimiter) TokenBucket(_ context.Context, _ TokenBucketRequest, _ limiters.TokenBucketStateBackend) *TokenBucketResponse {
	return &TokenBucketResponse{}
}

func (m *mockLimiter) Limit(_ context.Context, _ *limiters.TokenBucket) (*LimitResponse, error) {
	m.limitCalls++
	if m.limitErr != nil {
		return nil, m.limitErr
	}
	return &LimitResponse{}, nil
}

func newNextSpy() (http.Handler, *bool) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	return h, &called
}

func jsonRPCBody(method string) string {
	return `{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":[{}]}`
}

func errorCode(t *testing.T, body []byte) float64 {
	t.Helper()
	var resp struct {
		Error struct {
			Code float64 `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("could not parse error body %q: %v", string(body), err)
	}
	return resp.Error.Code
}

const testMethod = "EmailPasswordAuthenticator.Login"

func TestIPRateLimiter_LimitExhausted(t *testing.T) {
	mock := &mockLimiter{limitErr: limiters.ErrLimitExhausted}
	next, called := newNextSpy()
	h := NewIPRateLimiterMiddleware(mock, map[string]bool{testMethod: true}, 5, time.Second)(next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(jsonRPCBody(testMethod)))
	h.ServeHTTP(rec, req)

	if *called {
		t.Error("next handler should not be called when the limit is exhausted")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != jsonrpc.CodeTooManyRequests {
		t.Errorf("error code = %v, want %d", code, jsonrpc.CodeTooManyRequests)
	}
}

func TestIPRateLimiter_AllowsUnderLimit(t *testing.T) {
	mock := &mockLimiter{}
	next, called := newNextSpy()
	h := NewIPRateLimiterMiddleware(mock, map[string]bool{testMethod: true}, 5, time.Second)(next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(jsonRPCBody(testMethod)))
	h.ServeHTTP(rec, req)

	if !*called {
		t.Error("next handler should be called when under the limit")
	}
	if mock.limitCalls != 1 {
		t.Errorf("Limit calls = %d, want 1", mock.limitCalls)
	}
}

func TestIPRateLimiter_UntargetedMethodBypasses(t *testing.T) {
	mock := &mockLimiter{limitErr: limiters.ErrLimitExhausted}
	next, called := newNextSpy()
	h := NewIPRateLimiterMiddleware(mock, map[string]bool{testMethod: true}, 5, time.Second)(next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(jsonRPCBody("Wallet.SpendPoints")))
	h.ServeHTTP(rec, req)

	if !*called {
		t.Error("a non-targeted method must pass through")
	}
	if mock.limitCalls != 0 {
		t.Errorf("limiter must not be consulted for a non-targeted method; Limit calls = %d", mock.limitCalls)
	}
}

func TestIPRateLimiter_PreservesBodyForNext(t *testing.T) {
	mock := &mockLimiter{}
	var seen string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
		w.WriteHeader(http.StatusOK)
	})
	h := NewIPRateLimiterMiddleware(mock, map[string]bool{testMethod: true}, 5, time.Second)(next)

	body := jsonRPCBody(testMethod)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(body))
	h.ServeHTTP(rec, req)

	if seen != body {
		t.Errorf("downstream body = %q, want %q", seen, body)
	}
}

func TestUserRateLimiter_NoClaimBypasses(t *testing.T) {
	mock := &mockLimiter{limitErr: limiters.ErrLimitExhausted}
	next, called := newNextSpy()
	h := NewUserRateLimiterMiddleware(mock, 5, time.Second)(next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(jsonRPCBody(testMethod)))
	h.ServeHTTP(rec, req)

	if !*called {
		t.Error("request without a login claim must pass through")
	}
	if mock.limitCalls != 0 {
		t.Errorf("limiter must not be consulted without a claim; Limit calls = %d", mock.limitCalls)
	}
}

func TestUserRateLimiter_LimitExhausted(t *testing.T) {
	mock := &mockLimiter{limitErr: limiters.ErrLimitExhausted}
	next, called := newNextSpy()
	h := NewUserRateLimiterMiddleware(mock, 5, time.Second)(next)

	ctx := authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{UserID: "user-1"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(jsonRPCBody(testMethod))).WithContext(ctx)
	h.ServeHTTP(rec, req)

	if *called {
		t.Error("next handler should not be called when the limit is exhausted")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if code := errorCode(t, rec.Body.Bytes()); code != jsonrpc.CodeTooManyRequests {
		t.Errorf("error code = %v, want %d", code, jsonrpc.CodeTooManyRequests)
	}
}

func TestUserRateLimiter_AllowsUnderLimit(t *testing.T) {
	mock := &mockLimiter{}
	next, called := newNextSpy()
	h := NewUserRateLimiterMiddleware(mock, 5, time.Second)(next)

	ctx := authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{UserID: "user-1"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(jsonRPCBody(testMethod))).WithContext(ctx)
	h.ServeHTTP(rec, req)

	if !*called {
		t.Error("next handler should be called when under the limit")
	}
	if mock.limitCalls != 1 {
		t.Errorf("Limit calls = %d, want 1", mock.limitCalls)
	}
}

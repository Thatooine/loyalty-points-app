package authorization

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/jsonrpc"
	"github.com/Thatooine/loyalty-points-app/pkg/scope"
)

// fakeTokenService is a test double for authentication.AccessTokenValidator.
// ValidateAccessToken returns the configured claim, or err if set.
type fakeTokenService struct {
	claim authentication.LoginClaim
	err   error
}

func (f fakeTokenService) ValidateAccessToken(context.Context, authentication.ValidateAccessTokenRequest) (*authentication.ValidateAccessTokenResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &authentication.ValidateAccessTokenResponse{LoginClaim: f.claim}, nil
}

// rpcBody builds a JSON-RPC request body for the given method.
func rpcBody(method string) string {
	return `{"jsonrpc":"2.0","method":"` + method + `","params":[{}],"id":7}`
}

// errorResponse is the JSON-RPC error envelope the middleware writes on denial.
type errorResponse struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// run drives the middleware with a request body, an access-token service, and
// an optional bearer token. It returns whether next was reached, the recorded
// response, and the body the handler observed (to prove the body was restored).
func run(t *testing.T, accessTokenService authentication.AccessTokenValidator, policy *Policy, body, token string) (bool, *httptest.ResponseRecorder, string) {
	t.Helper()

	nextCalled := false
	var seenBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
	})

	handler := NewAuthorizationMiddleware(accessTokenService, policy)(next)

	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return nextCalled, rec, seenBody
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("could not decode error envelope: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

func memberTokens() fakeTokenService {
	return fakeTokenService{claim: authentication.LoginClaim{UserID: "u1", Permissions: []string{PermAccountReadOwn, PermWalletTransactOwn}}}
}

func TestAuthorizationMiddleware_PublicMethodBypassesAuth(t *testing.T) {
	policy := DefaultPolicy()
	// A token service that always errors — proves a public method never
	// touches authentication.
	tokens := fakeTokenService{err: errors.New("should not be called")}

	body := rpcBody(loginMethod)
	nextCalled, rec, seenBody := run(t, tokens, policy, body, "")

	if !nextCalled {
		t.Fatalf("public method did not reach next handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if seenBody != body {
		t.Fatalf("body not restored for handler: got %q", seenBody)
	}
}

func TestAuthorizationMiddleware_AllowsPermittedMethod(t *testing.T) {
	policy := DefaultPolicy()
	tokens := fakeTokenService{claim: authentication.LoginClaim{UserID: "u1", Permissions: []string{PermAccountReadOwn}}}

	body := rpcBody(getAccountMethod)
	nextCalled, rec, seenBody := run(t, tokens, policy, body, "valid-token")

	if !nextCalled {
		t.Fatalf("permitted method did not reach next handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if seenBody != body {
		t.Fatalf("body not restored for handler: got %q", seenBody)
	}
}

func TestAuthorizationMiddleware_PublishesEffectiveScope(t *testing.T) {
	policy := DefaultPolicy()
	tokens := fakeTokenService{claim: authentication.LoginClaim{UserID: "admin1", Permissions: []string{PermAccountReadAll}}}

	var gotScope scope.Scope
	var gotOK bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotScope, gotOK = scope.FromContext(r.Context())
	})
	handler := NewAuthorizationMiddleware(tokens, policy)(next)

	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(rpcBody(getAccountMethod)))
	req.Header.Set("Authorization", "Bearer valid-token")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !gotOK || gotScope != scope.All {
		t.Fatalf("effective scope in context = (%q, %v), want (%q, true)", gotScope, gotOK, scope.All)
	}
}

func TestAuthorizationMiddleware_DeniesUnpermittedMethod(t *testing.T) {
	policy := DefaultPolicy()
	tokens := memberTokens()

	nextCalled, rec, _ := run(t, tokens, policy, rpcBody(processTransactionBatchMethod), "valid-token")

	if nextCalled {
		t.Fatalf("denied method should not reach next handler")
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != jsonrpc.CodeForbidden {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, jsonrpc.CodeForbidden)
	}
	if string(resp.ID) != "7" {
		t.Fatalf("error id = %s, want 7 (echoed)", resp.ID)
	}
}

func TestAuthorizationMiddleware_MissingTokenIsUnauthorized(t *testing.T) {
	policy := DefaultPolicy()
	tokens := memberTokens()

	nextCalled, rec, _ := run(t, tokens, policy, rpcBody(getAccountMethod), "")

	if nextCalled {
		t.Fatalf("protected method without a token should not reach next handler")
	}
	if resp := decodeError(t, rec); resp.Error.Code != jsonrpc.CodeUnauthorized {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, jsonrpc.CodeUnauthorized)
	}
}

func TestAuthorizationMiddleware_InvalidTokenIsUnauthorized(t *testing.T) {
	policy := DefaultPolicy()
	tokens := fakeTokenService{err: errors.New("expired")}

	nextCalled, rec, _ := run(t, tokens, policy, rpcBody(getAccountMethod), "bad-token")

	if nextCalled {
		t.Fatalf("invalid token should not reach next handler")
	}
	if resp := decodeError(t, rec); resp.Error.Code != jsonrpc.CodeUnauthorized {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, jsonrpc.CodeUnauthorized)
	}
}

func TestAuthorizationMiddleware_MalformedBody(t *testing.T) {
	policy := DefaultPolicy()
	tokens := fakeTokenService{claim: authentication.LoginClaim{UserID: "u1", Permissions: []string{PermAccountReadAll}}}

	nextCalled, rec, _ := run(t, tokens, policy, "{not json", "valid-token")

	if nextCalled {
		t.Fatalf("malformed body should not reach next handler")
	}
	if resp := decodeError(t, rec); resp.Error.Code != jsonrpc.CodeParseError {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, jsonrpc.CodeParseError)
	}
}

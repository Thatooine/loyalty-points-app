package authorization

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

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

// run drives the middleware with a request body and (optionally) a login claim
// in context. It returns whether next was reached, the recorded response, and
// the body the handler observed (to prove the body was restored).
func run(t *testing.T, perms *Permissions, body string, claim *authentication.LoginClaim) (bool, *httptest.ResponseRecorder, string) {
	t.Helper()

	nextCalled := false
	var seenBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
	})

	handler := NewAuthorizationMiddleware(perms)(next)

	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(body))
	if claim != nil {
		req = req.WithContext(authentication.ContextWithLoginClaim(req.Context(), *claim))
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return nextCalled, rec, seenBody
}

func TestAuthorizationMiddleware_AllowsPermittedMethod(t *testing.T) {
	perms := NewPermissions(map[users.Role]map[string]bool{
		users.RoleMember: {"Wallet.GetByID": true},
	})
	claim := authentication.LoginClaim{UserID: "u1", Role: users.RoleMember}

	body := rpcBody("Wallet.GetByID")
	nextCalled, rec, seenBody := run(t, perms, body, &claim)

	if !nextCalled {
		t.Fatalf("permitted method did not reach next handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if seenBody != body {
		t.Fatalf("body not restored for handler: got %q, want %q", seenBody, body)
	}
}

func TestAuthorizationMiddleware_AdminWildcard(t *testing.T) {
	perms := DefaultPermissions()
	claim := authentication.LoginClaim{UserID: "admin1", Role: users.RoleAdmin}

	nextCalled, rec, _ := run(t, perms, rpcBody("Wallet.ProcessTransaction"), &claim)

	if !nextCalled || rec.Code != http.StatusOK {
		t.Fatalf("admin should be allowed: nextCalled=%v status=%d", nextCalled, rec.Code)
	}
}

func TestAuthorizationMiddleware_DeniesUnpermittedMethod(t *testing.T) {
	perms := NewPermissions(map[users.Role]map[string]bool{
		users.RoleMember: {"Wallet.GetByID": true},
	})
	claim := authentication.LoginClaim{UserID: "u1", Role: users.RoleMember}

	nextCalled, rec, _ := run(t, perms, rpcBody("Wallet.ProcessTransaction"), &claim)

	if nextCalled {
		t.Fatalf("denied method should not reach next handler")
	}
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("could not decode error envelope: %v (body=%s)", err, rec.Body.String())
	}
	if resp.Error.Code != codeForbidden {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, codeForbidden)
	}
	if string(resp.ID) != "7" {
		t.Fatalf("error id = %s, want 7 (echoed)", resp.ID)
	}
}

func TestAuthorizationMiddleware_NoClaimIsUnauthorized(t *testing.T) {
	perms := DefaultPermissions()

	nextCalled, rec, _ := run(t, perms, rpcBody("Wallet.GetByID"), nil)

	if nextCalled {
		t.Fatalf("request without a claim should not reach next handler")
	}
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("could not decode error envelope: %v", err)
	}
	if resp.Error.Code != codeUnauthorized {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, codeUnauthorized)
	}
}

func TestAuthorizationMiddleware_MalformedBody(t *testing.T) {
	perms := DefaultPermissions()
	claim := authentication.LoginClaim{UserID: "u1", Role: users.RoleAdmin}

	nextCalled, rec, _ := run(t, perms, "{not json", &claim)

	if nextCalled {
		t.Fatalf("malformed body should not reach next handler")
	}
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("could not decode error envelope: %v", err)
	}
	if resp.Error.Code != codeParseError {
		t.Fatalf("error code = %d, want %d", resp.Error.Code, codeParseError)
	}
}

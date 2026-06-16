package authentication

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

// stubLogout is a minimal LogoutService double for the adaptor tests.
type stubLogout struct {
	called  bool
	gotUser string
	err     error
}

func (s *stubLogout) Logout(ctx context.Context, request LogoutRequest) (*LogoutResponse, error) {
	s.called = true
	s.gotUser = request.UserID
	if s.err != nil {
		return nil, s.err
	}
	return &LogoutResponse{TokenVersion: 1}, nil
}

// TestLogoutAdaptor_UsesClaimUserID confirms the adaptor takes the acting user
// from the verified login claim (never the request body) and reports success.
func TestLogoutAdaptor_UsesClaimUserID(t *testing.T) {
	stub := &stubLogout{}
	adaptor := NewLogoutServiceJSONRPCAdaptor(stub)

	ctx := ContextWithLoginClaim(context.Background(), LoginClaim{UserID: "user-9"})
	req := httptest.NewRequest("POST", "/api", nil).WithContext(ctx)

	var result LogoutJSONRPCResponse
	if err := adaptor.Logout(req, &LogoutJSONRPCRequest{}, &result); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if !stub.called {
		t.Fatal("Logout service was not called")
	}
	if stub.gotUser != "user-9" {
		t.Errorf("service called with UserID = %q, want user-9", stub.gotUser)
	}
	if !result.OK {
		t.Error("result.OK = false, want true")
	}
}

// TestLogoutAdaptor_NoClaim confirms the adaptor rejects a request with no login
// claim on the context and never reaches the service.
func TestLogoutAdaptor_NoClaim(t *testing.T) {
	stub := &stubLogout{}
	adaptor := NewLogoutServiceJSONRPCAdaptor(stub)

	req := httptest.NewRequest("POST", "/api", nil) // no claim on context

	var result LogoutJSONRPCResponse
	if err := adaptor.Logout(req, &LogoutJSONRPCRequest{}, &result); err == nil {
		t.Fatal("Logout() without claim: expected error, got nil")
	}
	if stub.called {
		t.Error("Logout service must not be called when unauthenticated")
	}
}

// TestLogoutAdaptor_ServiceError confirms a service error surfaces as a wire
// error rather than a false success.
func TestLogoutAdaptor_ServiceError(t *testing.T) {
	stub := &stubLogout{err: errors.New("boom")}
	adaptor := NewLogoutServiceJSONRPCAdaptor(stub)

	ctx := ContextWithLoginClaim(context.Background(), LoginClaim{UserID: "user-9"})
	req := httptest.NewRequest("POST", "/api", nil).WithContext(ctx)

	var result LogoutJSONRPCResponse
	if err := adaptor.Logout(req, &LogoutJSONRPCRequest{}, &result); err == nil {
		t.Fatal("Logout() with service error: expected error, got nil")
	}
	if result.OK {
		t.Error("result.OK = true on error, want false")
	}
}

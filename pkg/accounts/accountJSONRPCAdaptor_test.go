package accounts_test

import (
	"context"
	"net/http/httptest"
	"testing"

	internalaccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

// TestGetByID_HappyPath: with a valid claim, the adaptor scopes the lookup to
// the caller's user id and maps the repository account onto the wire result.
func TestGetByID_HappyPath(t *testing.T) {
	const userID = "user-1"
	mock := &internalaccounts.MockAccountRepository{T: t}
	mock.GetByIDFunc = func(t *testing.T, m *internalaccounts.MockAccountRepository, ctx context.Context, request accounts.GetAccountByIDRequest) (*accounts.GetAccountByIDResponse, error) {
		// The adaptor must pin the scope to the claim's user id, never the wire.
		if request.UserID != userID {
			t.Errorf("repository called with UserID = %q, want %q", request.UserID, userID)
		}
		if request.AccountID != "acc-1" {
			t.Errorf("repository called with AccountID = %q, want %q", request.AccountID, "acc-1")
		}
		return &accounts.GetAccountByIDResponse{Account: accounts.Account{
			ID:      "acc-1",
			OwnerID: userID,
			Name:    "My Wallet",
			Balance: 250,
		}}, nil
	}

	adaptor := accounts.NewAccountJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil).WithContext(
		authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{
			UserID: userID,
			Role:   users.RoleMember,
		}),
	)

	var result accounts.AccountResult
	if err := adaptor.GetByID(req, &accounts.GetByIDParams{AccountID: "acc-1"}, &result); err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}

	if result.ID != "acc-1" {
		t.Errorf("result.ID = %q, want %q", result.ID, "acc-1")
	}
	if result.UserID != userID {
		t.Errorf("result.UserID = %q, want %q", result.UserID, userID)
	}
	if result.Name != "My Wallet" {
		t.Errorf("result.Name = %q, want %q", result.Name, "My Wallet")
	}
	if result.Balance != 250 {
		t.Errorf("result.Balance = %d, want 250", result.Balance)
	}
}

// TestGetByID_NotFound: a repository ErrNotFound (missing or unowned account) is
// mapped to the opaque "account not found" wire error — no existence leak.
func TestGetByID_NotFound(t *testing.T) {
	mock := &internalaccounts.MockAccountRepository{T: t}
	mock.GetByIDFunc = func(t *testing.T, m *internalaccounts.MockAccountRepository, ctx context.Context, request accounts.GetAccountByIDRequest) (*accounts.GetAccountByIDResponse, error) {
		return nil, errs.ErrNotFound
	}

	adaptor := accounts.NewAccountJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil).WithContext(
		authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{
			UserID: "user-1",
			Role:   users.RoleMember,
		}),
	)

	var result accounts.AccountResult
	err := adaptor.GetByID(req, &accounts.GetByIDParams{AccountID: "acc-1"}, &result)
	if err == nil {
		t.Fatal("GetByID: expected an error, got nil")
	}
	if err.Error() != "account not found" {
		t.Errorf("error = %q, want %q", err.Error(), "account not found")
	}
}

// TestGetByID_Unauthenticated: with no login claim on the context the adaptor
// fails closed and never touches the repository.
func TestGetByID_Unauthenticated(t *testing.T) {
	mock := &internalaccounts.MockAccountRepository{T: t}
	mock.GetByIDFunc = func(t *testing.T, m *internalaccounts.MockAccountRepository, ctx context.Context, request accounts.GetAccountByIDRequest) (*accounts.GetAccountByIDResponse, error) {
		t.Fatal("repository must not be called without a claim")
		return nil, nil
	}

	adaptor := accounts.NewAccountJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil) // no claim on context

	var result accounts.AccountResult
	if err := adaptor.GetByID(req, &accounts.GetByIDParams{AccountID: "acc-1"}, &result); err == nil {
		t.Fatal("GetByID: expected unauthorized error, got nil")
	}
}

// TestUpdateAccountBalance_MemberForbidden: the raw balance write is admin-only.
// A member claim is rejected by the adaptor's defence-in-depth role check before
// the repository is reached.
func TestUpdateAccountBalance_MemberForbidden(t *testing.T) {
	mock := &internalaccounts.MockAccountRepository{T: t}
	mock.UpdateAccountBalanceFunc = func(t *testing.T, m *internalaccounts.MockAccountRepository, ctx context.Context, request accounts.UpdateAccountBalanceRequest) (*accounts.UpdateAccountBalanceResponse, error) {
		t.Fatal("repository must not be called for a non-admin caller")
		return nil, nil
	}

	adaptor := accounts.NewAccountJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil).WithContext(
		authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{
			UserID: "user-1",
			Role:   users.RoleMember,
		}),
	)

	var result accounts.BalanceResult
	if err := adaptor.UpdateAccountBalance(req, &accounts.UpdateAccountBalanceParams{AccountID: "acc-1", Delta: 100}, &result); err == nil {
		t.Fatal("UpdateAccountBalance as member: expected forbidden error, got nil")
	}
}

// TestUpdateAccountBalance_AdminHappyPath: an admin claim passes the role gate
// and the adaptor returns the post-delta balance from the repository.
func TestUpdateAccountBalance_AdminHappyPath(t *testing.T) {
	mock := &internalaccounts.MockAccountRepository{T: t}
	mock.UpdateAccountBalanceFunc = func(t *testing.T, m *internalaccounts.MockAccountRepository, ctx context.Context, request accounts.UpdateAccountBalanceRequest) (*accounts.UpdateAccountBalanceResponse, error) {
		if request.AccountID != "acc-1" || request.Delta != 500 {
			t.Errorf("repository called with %+v, want AccountID=acc-1 Delta=500", request)
		}
		return &accounts.UpdateAccountBalanceResponse{Balance: 500}, nil
	}

	adaptor := accounts.NewAccountJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil).WithContext(
		authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{
			UserID: "admin-1",
			Role:   users.RoleAdmin,
		}),
	)

	var result accounts.BalanceResult
	if err := adaptor.UpdateAccountBalance(req, &accounts.UpdateAccountBalanceParams{AccountID: "acc-1", Delta: 500}, &result); err != nil {
		t.Fatalf("UpdateAccountBalance returned error: %v", err)
	}
	if result.Balance != 500 {
		t.Errorf("result.Balance = %d, want 500", result.Balance)
	}
	if result.AccountID != "acc-1" {
		t.Errorf("result.AccountID = %q, want %q", result.AccountID, "acc-1")
	}
}

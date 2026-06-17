package accounts

import (
	"context"
	"errors"
	"testing"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// TestServiceGetAccountByID_HappyPath proves the service forwards the scoped request to
// the repository and returns the account it gets back.
func TestServiceGetAccountByID_HappyPath(t *testing.T) {
	var captured pkgAccounts.GetAccountByIDRequest
	repo := &MockAccountRepository{
		T: t,
		GetByIDFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error) {
			captured = request
			return &pkgAccounts.GetAccountByIDResponse{Account: pkgAccounts.Account{ID: "acc-1", OwnerID: "user-1", Name: "Wallet", Balance: 250}}, nil
		},
	}
	service := NewAccountServiceImpl(repo)

	resp, err := service.GetAccountByID(context.Background(), pkgAccounts.ReadAccountRequest{AccountID: "acc-1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("GetAccountByID() error = %v, want nil", err)
	}
	if resp.Account.ID != "acc-1" {
		t.Errorf("returned ID = %q, want %q", resp.Account.ID, "acc-1")
	}
	if captured.AccountID != "acc-1" || captured.UserID != "user-1" {
		t.Errorf("repo called with %+v, want AccountID=acc-1 UserID=user-1", captured)
	}
}

// TestServiceGetAccountByID_ValidationFailsClosed proves an invalid request is rejected
// before any persistence: the repository is never reached.
func TestServiceGetAccountByID_ValidationFailsClosed(t *testing.T) {
	repo := &MockAccountRepository{
		T: t,
		GetByIDFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error) {
			t.Fatal("repository must not be called when the request is invalid")
			return nil, nil
		},
	}
	service := NewAccountServiceImpl(repo)

	_, err := service.GetAccountByID(context.Background(), pkgAccounts.ReadAccountRequest{AccountID: "", UserID: "user-1"})
	if err == nil {
		t.Fatal("GetAccountByID() error = nil, want validation error")
	}
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Errorf("GetAccountByID() error = %v, want it to wrap %v", err, errs.ErrInvalidArgument)
	}
}

// TestServiceGetAccountByID_RepositoryError proves a repository failure surfaces and the
// underlying sentinel is preserved through the wrap.
func TestServiceGetAccountByID_RepositoryError(t *testing.T) {
	repo := &MockAccountRepository{
		T: t,
		GetByIDFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error) {
			return nil, errs.ErrNotFound
		},
	}
	service := NewAccountServiceImpl(repo)

	_, err := service.GetAccountByID(context.Background(), pkgAccounts.ReadAccountRequest{AccountID: "acc-1", UserID: "user-1"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("GetAccountByID() error = %v, want it to wrap %v", err, errs.ErrNotFound)
	}
}

// TestServiceGetAccountBalance_HappyPath proves the balance read is forwarded and
// returned.
func TestServiceGetAccountBalance_HappyPath(t *testing.T) {
	var captured pkgAccounts.GetAccountBalanceRequest
	repo := &MockAccountRepository{
		T: t,
		GetAccountBalanceFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountBalanceRequest) (*pkgAccounts.GetAccountBalanceResponse, error) {
			captured = request
			return &pkgAccounts.GetAccountBalanceResponse{Balance: 999}, nil
		},
	}
	service := NewAccountServiceImpl(repo)

	resp, err := service.GetAccountBalance(context.Background(), pkgAccounts.ReadAccountBalanceRequest{AccountID: "acc-1", UserID: "user-1"})
	if err != nil {
		t.Fatalf("GetAccountBalance() error = %v, want nil", err)
	}
	if resp.Balance != 999 {
		t.Errorf("returned Balance = %d, want 999", resp.Balance)
	}
	if captured.AccountID != "acc-1" || captured.UserID != "user-1" {
		t.Errorf("repo called with %+v, want AccountID=acc-1 UserID=user-1", captured)
	}
}

// TestServiceGetAccountBalance_RepositoryError proves a repository failure
// surfaces with its sentinel intact.
func TestServiceGetAccountBalance_RepositoryError(t *testing.T) {
	repo := &MockAccountRepository{
		T: t,
		GetAccountBalanceFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountBalanceRequest) (*pkgAccounts.GetAccountBalanceResponse, error) {
			return nil, errs.ErrNotFound
		},
	}
	service := NewAccountServiceImpl(repo)

	_, err := service.GetAccountBalance(context.Background(), pkgAccounts.ReadAccountBalanceRequest{AccountID: "acc-1", UserID: "user-1"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("GetAccountBalance() error = %v, want it to wrap %v", err, errs.ErrNotFound)
	}
}

// TestServiceUpdateAccountName_HappyPath proves the rename is forwarded with all
// fields and the renamed account is returned.
func TestServiceUpdateAccountName_HappyPath(t *testing.T) {
	var captured pkgAccounts.UpdateAccountNameRequest
	repo := &MockAccountRepository{
		T: t,
		UpdateAccountNameFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.UpdateAccountNameRequest) (*pkgAccounts.UpdateAccountNameResponse, error) {
			captured = request
			return &pkgAccounts.UpdateAccountNameResponse{Account: pkgAccounts.Account{ID: request.AccountID, OwnerID: request.UserID, Name: request.Name}}, nil
		},
	}
	service := NewAccountServiceImpl(repo)

	resp, err := service.UpdateAccountName(context.Background(), pkgAccounts.RenameAccountRequest{AccountID: "acc-1", Name: "Renamed", UserID: "user-1"})
	if err != nil {
		t.Fatalf("UpdateAccountName() error = %v, want nil", err)
	}
	if resp.Account.Name != "Renamed" {
		t.Errorf("returned Name = %q, want %q", resp.Account.Name, "Renamed")
	}
	if captured.AccountID != "acc-1" || captured.Name != "Renamed" || captured.UserID != "user-1" {
		t.Errorf("repo called with %+v, want AccountID=acc-1 Name=Renamed UserID=user-1", captured)
	}
}

// TestServiceUpdateAccountName_ValidationFailsClosed proves a blank name is
// rejected before the repository is reached.
func TestServiceUpdateAccountName_ValidationFailsClosed(t *testing.T) {
	repo := &MockAccountRepository{
		T: t,
		UpdateAccountNameFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.UpdateAccountNameRequest) (*pkgAccounts.UpdateAccountNameResponse, error) {
			t.Fatal("repository must not be called when the request is invalid")
			return nil, nil
		},
	}
	service := NewAccountServiceImpl(repo)

	_, err := service.UpdateAccountName(context.Background(), pkgAccounts.RenameAccountRequest{AccountID: "acc-1", Name: "", UserID: "user-1"})
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Errorf("UpdateAccountName() error = %v, want it to wrap %v", err, errs.ErrInvalidArgument)
	}
}

// TestServiceUpdateAccountBalance_HappyPath proves the signed delta is forwarded
// and the post-delta balance is returned.
func TestServiceUpdateAccountBalance_HappyPath(t *testing.T) {
	var captured pkgAccounts.UpdateAccountBalanceRequest
	repo := &MockAccountRepository{
		T: t,
		UpdateAccountBalanceFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.UpdateAccountBalanceRequest) (*pkgAccounts.UpdateAccountBalanceResponse, error) {
			captured = request
			return &pkgAccounts.UpdateAccountBalanceResponse{Balance: 600}, nil
		},
	}
	service := NewAccountServiceImpl(repo)

	resp, err := service.UpdateAccountBalance(context.Background(), pkgAccounts.AdjustAccountBalanceRequest{AccountID: "acc-1", Delta: 100, UserID: "user-1"})
	if err != nil {
		t.Fatalf("UpdateAccountBalance() error = %v, want nil", err)
	}
	if resp.Balance != 600 {
		t.Errorf("returned Balance = %d, want 600", resp.Balance)
	}
	if captured.AccountID != "acc-1" || captured.Delta != 100 || captured.UserID != "user-1" {
		t.Errorf("repo called with %+v, want AccountID=acc-1 Delta=100 UserID=user-1", captured)
	}
}

// TestServiceUpdateAccountBalance_RepositoryError proves an insufficient-balance
// failure surfaces with its sentinel intact.
func TestServiceUpdateAccountBalance_RepositoryError(t *testing.T) {
	repo := &MockAccountRepository{
		T: t,
		UpdateAccountBalanceFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.UpdateAccountBalanceRequest) (*pkgAccounts.UpdateAccountBalanceResponse, error) {
			return nil, errs.ErrInsufficientBalance
		},
	}
	service := NewAccountServiceImpl(repo)

	_, err := service.UpdateAccountBalance(context.Background(), pkgAccounts.AdjustAccountBalanceRequest{AccountID: "acc-1", Delta: -100, UserID: "user-1"})
	if !errors.Is(err, errs.ErrInsufficientBalance) {
		t.Errorf("UpdateAccountBalance() error = %v, want it to wrap %v", err, errs.ErrInsufficientBalance)
	}
}

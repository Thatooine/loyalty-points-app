package accounts

import (
	"context"
	"errors"
	"testing"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

func TestOpenAccount_HappyPath(t *testing.T) {
	var captured pkgAccounts.CreateAccountRequest
	repo := &MockAccountRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
			captured = request
			created := request.Account
			created.ID = "acc-1"
			return &pkgAccounts.CreateAccountResponse{Account: created}, nil
		},
	}
	service := NewAccountOpenerServiceImpl(repo)

	resp, err := service.OpenAccount(context.Background(), pkgAccounts.OpenAccountRequest{
		UserID: "user-1",
		Name:   "Savings",
	})
	if err != nil {
		t.Fatalf("OpenAccount() error = %v, want nil", err)
	}

	if resp.Account.ID != "acc-1" {
		t.Errorf("returned ID = %q, want %q", resp.Account.ID, "acc-1")
	}
	if resp.Account.Name != "Savings" {
		t.Errorf("returned Name = %q, want %q", resp.Account.Name, "Savings")
	}
	if resp.Account.OwnerID != "user-1" {
		t.Errorf("returned OwnerID = %q, want %q", resp.Account.OwnerID, "user-1")
	}
	if resp.Account.Balance != 0 {
		t.Errorf("returned Balance = %d, want 0", resp.Account.Balance)
	}

	if captured.Account.OwnerID != "user-1" {
		t.Errorf("Create called with OwnerID = %q, want caller %q", captured.Account.OwnerID, "user-1")
	}
	if captured.Account.Name != "Savings" {
		t.Errorf("Create called with Name = %q, want %q", captured.Account.Name, "Savings")
	}
	if captured.Account.Balance != 0 {
		t.Errorf("Create called with Balance = %d, want 0", captured.Account.Balance)
	}
	if captured.Account.CreatedAt.IsZero() {
		t.Error("Create called with zero CreatedAt, want a stamped time")
	}
}

func TestOpenAccount_DefaultName(t *testing.T) {
	var captured pkgAccounts.CreateAccountRequest
	repo := &MockAccountRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
			captured = request
			return &pkgAccounts.CreateAccountResponse{Account: request.Account}, nil
		},
	}
	service := NewAccountOpenerServiceImpl(repo)

	resp, err := service.OpenAccount(context.Background(), pkgAccounts.OpenAccountRequest{
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("OpenAccount() error = %v, want nil", err)
	}

	if captured.Account.Name != defaultAccountName {
		t.Errorf("Create called with Name = %q, want default %q", captured.Account.Name, defaultAccountName)
	}
	if resp.Account.Name != defaultAccountName {
		t.Errorf("returned Name = %q, want default %q", resp.Account.Name, defaultAccountName)
	}
}

func TestOpenAccount_ValidationFailsClosed(t *testing.T) {
	repo := &MockAccountRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
			t.Fatal("Create must not be called when the request is invalid")
			return nil, nil
		},
	}
	service := NewAccountOpenerServiceImpl(repo)

	_, err := service.OpenAccount(context.Background(), pkgAccounts.OpenAccountRequest{
		UserID: "",
		Name:   "Savings",
	})
	if err == nil {
		t.Fatal("OpenAccount() error = nil, want validation error")
	}
}

func TestOpenAccount_RepositoryError(t *testing.T) {
	repo := &MockAccountRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
			return nil, errs.ErrAlreadyExists
		},
	}
	service := NewAccountOpenerServiceImpl(repo)

	resp, err := service.OpenAccount(context.Background(), pkgAccounts.OpenAccountRequest{
		UserID: "user-1",
		Name:   "Savings",
	})
	if err == nil {
		t.Fatal("OpenAccount() error = nil, want repository error")
	}
	if !errors.Is(err, errs.ErrAlreadyExists) {
		t.Errorf("OpenAccount() error = %v, want it to wrap %v", err, errs.ErrAlreadyExists)
	}
	if resp != nil {
		t.Errorf("OpenAccount() resp = %+v, want nil on error", resp)
	}
}

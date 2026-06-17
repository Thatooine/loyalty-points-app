package accounts

import (
	"context"
	"testing"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
)

// Ensure that MockAccountService implements the AccountService interface.
var _ pkgAccounts.AccountService = &MockAccountService{}

// MockAccountService is a hand-written mock implementation of
// accounts.AccountService. Each method delegates to a function field that a test
// sets to control the return value (the happy path) or the error (the failure
// path); an unset field is a no-op returning the zero value, so a test only
// wires the methods it exercises.
type MockAccountService struct {
	T *testing.T

	GetAccountByIDFunc       func(t *testing.T, m *MockAccountService, ctx context.Context, request pkgAccounts.ReadAccountRequest) (*pkgAccounts.ReadAccountResponse, error)
	GetAccountBalanceFunc    func(t *testing.T, m *MockAccountService, ctx context.Context, request pkgAccounts.ReadAccountBalanceRequest) (*pkgAccounts.ReadAccountBalanceResponse, error)
	UpdateAccountNameFunc    func(t *testing.T, m *MockAccountService, ctx context.Context, request pkgAccounts.RenameAccountRequest) (*pkgAccounts.RenameAccountResponse, error)
	UpdateAccountBalanceFunc func(t *testing.T, m *MockAccountService, ctx context.Context, request pkgAccounts.AdjustAccountBalanceRequest) (*pkgAccounts.AdjustAccountBalanceResponse, error)
}

func (m *MockAccountService) GetAccountByID(ctx context.Context, request pkgAccounts.ReadAccountRequest) (*pkgAccounts.ReadAccountResponse, error) {
	if m.GetAccountByIDFunc == nil {
		return nil, nil
	}
	return m.GetAccountByIDFunc(m.T, m, ctx, request)
}

func (m *MockAccountService) GetAccountBalance(ctx context.Context, request pkgAccounts.ReadAccountBalanceRequest) (*pkgAccounts.ReadAccountBalanceResponse, error) {
	if m.GetAccountBalanceFunc == nil {
		return nil, nil
	}
	return m.GetAccountBalanceFunc(m.T, m, ctx, request)
}

func (m *MockAccountService) UpdateAccountName(ctx context.Context, request pkgAccounts.RenameAccountRequest) (*pkgAccounts.RenameAccountResponse, error) {
	if m.UpdateAccountNameFunc == nil {
		return nil, nil
	}
	return m.UpdateAccountNameFunc(m.T, m, ctx, request)
}

func (m *MockAccountService) UpdateAccountBalance(ctx context.Context, request pkgAccounts.AdjustAccountBalanceRequest) (*pkgAccounts.AdjustAccountBalanceResponse, error) {
	if m.UpdateAccountBalanceFunc == nil {
		return nil, nil
	}
	return m.UpdateAccountBalanceFunc(m.T, m, ctx, request)
}

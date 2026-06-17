package accounts

import (
	"context"
	"testing"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
)

var _ pkgAccounts.AccountRepository = &MockAccountRepository{}

type MockAccountRepository struct {
	T *testing.T

	CreateFunc               func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error)
	ListFunc                 func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.ListAccountsRequest) (*pkgAccounts.ListAccountsResponse, error)
	GetByIDFunc              func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error)
	GetAccountBalanceFunc    func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountBalanceRequest) (*pkgAccounts.GetAccountBalanceResponse, error)
	UpdateAccountBalanceFunc func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.UpdateAccountBalanceRequest) (*pkgAccounts.UpdateAccountBalanceResponse, error)
	UpdateAccountNameFunc    func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.UpdateAccountNameRequest) (*pkgAccounts.UpdateAccountNameResponse, error)
}

func (m *MockAccountRepository) Create(ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
	if m.CreateFunc == nil {
		return nil, nil
	}
	return m.CreateFunc(m.T, m, ctx, request)
}

func (m *MockAccountRepository) List(ctx context.Context, request pkgAccounts.ListAccountsRequest) (*pkgAccounts.ListAccountsResponse, error) {
	if m.ListFunc == nil {
		return nil, nil
	}
	return m.ListFunc(m.T, m, ctx, request)
}

func (m *MockAccountRepository) GetByID(ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error) {
	if m.GetByIDFunc == nil {
		return nil, nil
	}
	return m.GetByIDFunc(m.T, m, ctx, request)
}

func (m *MockAccountRepository) GetAccountBalance(ctx context.Context, request pkgAccounts.GetAccountBalanceRequest) (*pkgAccounts.GetAccountBalanceResponse, error) {
	if m.GetAccountBalanceFunc == nil {
		return nil, nil
	}
	return m.GetAccountBalanceFunc(m.T, m, ctx, request)
}

func (m *MockAccountRepository) UpdateAccountBalance(ctx context.Context, request pkgAccounts.UpdateAccountBalanceRequest) (*pkgAccounts.UpdateAccountBalanceResponse, error) {
	if m.UpdateAccountBalanceFunc == nil {
		return nil, nil
	}
	return m.UpdateAccountBalanceFunc(m.T, m, ctx, request)
}

func (m *MockAccountRepository) UpdateAccountName(ctx context.Context, request pkgAccounts.UpdateAccountNameRequest) (*pkgAccounts.UpdateAccountNameResponse, error) {
	if m.UpdateAccountNameFunc == nil {
		return nil, nil
	}
	return m.UpdateAccountNameFunc(m.T, m, ctx, request)
}

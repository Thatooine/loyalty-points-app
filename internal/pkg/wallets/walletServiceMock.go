package wallets

import (
	"context"
	"testing"

	pkgWallets "github.com/Thatooine/loyalty-points-app/pkg/wallets"
)

// Ensure that MockWalletService implements the WalletService interface.
var _ pkgWallets.WalletService = &MockWalletService{}

// MockWalletService is a hand-written mock of wallets.WalletService. Services
// that compose the wallet service (e.g. RewardRedeemer) drive this in their unit
// tests: each method delegates to a function field a test sets; an unset field
// is a no-op returning the zero value, so a test only wires the methods it
// exercises.
type MockWalletService struct {
	T *testing.T

	ProcessTransactionFunc      func(t *testing.T, m *MockWalletService, ctx context.Context, request pkgWallets.ProcessTransactionRequest) (*pkgWallets.ProcessTransactionResponse, error)
	EarnPointsFunc              func(t *testing.T, m *MockWalletService, ctx context.Context, request pkgWallets.EarnPointsRequest) (*pkgWallets.ProcessTransactionResponse, error)
	SpendPointsFunc             func(t *testing.T, m *MockWalletService, ctx context.Context, request pkgWallets.SpendPointsRequest) (*pkgWallets.ProcessTransactionResponse, error)
	ProcessTransactionBatchFunc func(t *testing.T, m *MockWalletService, ctx context.Context, request pkgWallets.ProcessTransactionBatchRequest) (*pkgWallets.ProcessTransactionBatchResponse, error)
}

func (m *MockWalletService) ProcessTransaction(ctx context.Context, request pkgWallets.ProcessTransactionRequest) (*pkgWallets.ProcessTransactionResponse, error) {
	if m.ProcessTransactionFunc == nil {
		return nil, nil
	}
	return m.ProcessTransactionFunc(m.T, m, ctx, request)
}

func (m *MockWalletService) EarnPoints(ctx context.Context, request pkgWallets.EarnPointsRequest) (*pkgWallets.ProcessTransactionResponse, error) {
	if m.EarnPointsFunc == nil {
		return nil, nil
	}
	return m.EarnPointsFunc(m.T, m, ctx, request)
}

func (m *MockWalletService) SpendPoints(ctx context.Context, request pkgWallets.SpendPointsRequest) (*pkgWallets.ProcessTransactionResponse, error) {
	if m.SpendPointsFunc == nil {
		return nil, nil
	}
	return m.SpendPointsFunc(m.T, m, ctx, request)
}

func (m *MockWalletService) ProcessTransactionBatch(ctx context.Context, request pkgWallets.ProcessTransactionBatchRequest) (*pkgWallets.ProcessTransactionBatchResponse, error) {
	if m.ProcessTransactionBatchFunc == nil {
		return nil, nil
	}
	return m.ProcessTransactionBatchFunc(m.T, m, ctx, request)
}

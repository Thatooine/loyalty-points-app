package wallets

import (
	"context"
	"testing"

	pkgWallets "github.com/Thatooine/loyalty-points-app/pkg/wallets"
)

var _ pkgWallets.WalletService = &MockWalletService{}

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

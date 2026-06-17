package audit

import (
	"context"
	"testing"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
)

// Ensure that MockAuditEntryRepository implements the AuditEntryRepository interface.
var _ pkgAudit.AuditEntryRepository = &MockAuditEntryRepository{}

// MockAuditEntryRepository is a hand-written mock implementation of
// audits.AuditEntryRepository. Each method delegates to a function field that a
// test sets to control the return value (the happy path) or the error (the
// failure path); an unset field is a no-op returning the zero value, so a test
// only wires the methods it exercises.
type MockAuditEntryRepository struct {
	T *testing.T

	CreateFunc               func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.CreateAuditEntryRequest) (*pkgAudit.CreateAuditEntryResponse, error)
	ListFunc                 func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.ListAuditEntriesRequest) (*pkgAudit.ListAuditEntriesResponse, error)
	ListByTransactionRefFunc func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.ListAuditEntriesByTransactionRefRequest) (*pkgAudit.ListAuditEntriesByTransactionRefResponse, error)
	ListByAccountIDFunc      func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.ListAuditEntriesByAccountIDRequest) (*pkgAudit.ListAuditEntriesByAccountIDResponse, error)
	GetByIDFunc              func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.GetAuditEntryByIDRequest) (*pkgAudit.GetAuditEntryByIDResponse, error)
}

func (m *MockAuditEntryRepository) Create(ctx context.Context, request pkgAudit.CreateAuditEntryRequest) (*pkgAudit.CreateAuditEntryResponse, error) {
	if m.CreateFunc == nil {
		return nil, nil
	}
	return m.CreateFunc(m.T, m, ctx, request)
}

func (m *MockAuditEntryRepository) List(ctx context.Context, request pkgAudit.ListAuditEntriesRequest) (*pkgAudit.ListAuditEntriesResponse, error) {
	if m.ListFunc == nil {
		return nil, nil
	}
	return m.ListFunc(m.T, m, ctx, request)
}

func (m *MockAuditEntryRepository) ListByTransactionRef(ctx context.Context, request pkgAudit.ListAuditEntriesByTransactionRefRequest) (*pkgAudit.ListAuditEntriesByTransactionRefResponse, error) {
	if m.ListByTransactionRefFunc == nil {
		return nil, nil
	}
	return m.ListByTransactionRefFunc(m.T, m, ctx, request)
}

func (m *MockAuditEntryRepository) ListByAccountID(ctx context.Context, request pkgAudit.ListAuditEntriesByAccountIDRequest) (*pkgAudit.ListAuditEntriesByAccountIDResponse, error) {
	if m.ListByAccountIDFunc == nil {
		return nil, nil
	}
	return m.ListByAccountIDFunc(m.T, m, ctx, request)
}

func (m *MockAuditEntryRepository) GetByID(ctx context.Context, request pkgAudit.GetAuditEntryByIDRequest) (*pkgAudit.GetAuditEntryByIDResponse, error) {
	if m.GetByIDFunc == nil {
		return nil, nil
	}
	return m.GetByIDFunc(m.T, m, ctx, request)
}

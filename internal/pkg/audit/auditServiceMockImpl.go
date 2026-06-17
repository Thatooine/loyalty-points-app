package audit

import (
	"context"
	"testing"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
)

// Ensure that MockAuditService implements the AuditService interface.
var _ pkgAudit.AuditService = &MockAuditService{}

// MockAuditService is a hand-written mock implementation of audits.AuditService.
// Each method delegates to a function field that a test sets to control the
// return value (the happy path) or the error (the failure path); an unset field
// is a no-op returning the zero value, so a test only wires the methods it
// exercises.
type MockAuditService struct {
	T *testing.T

	ListByTransactionRefFunc func(t *testing.T, m *MockAuditService, ctx context.Context, request pkgAudit.ListAuditByRefRequest) (*pkgAudit.ListAuditByRefResponse, error)
}

func (m *MockAuditService) ListByTransactionRef(ctx context.Context, request pkgAudit.ListAuditByRefRequest) (*pkgAudit.ListAuditByRefResponse, error) {
	if m.ListByTransactionRefFunc == nil {
		return nil, nil
	}
	return m.ListByTransactionRefFunc(m.T, m, ctx, request)
}

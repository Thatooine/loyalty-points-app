package audit

import (
	"context"
	"testing"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
)

var _ pkgAudit.AuditService = &MockAuditService{}

type MockAuditService struct {
	T *testing.T

	FetchTransactionAuditTrailFunc func(t *testing.T, m *MockAuditService, ctx context.Context, request pkgAudit.ListAuditByRefRequest) (*pkgAudit.ListAuditByRefResponse, error)
}

func (m *MockAuditService) FetchTransactionAuditTrail(ctx context.Context, request pkgAudit.ListAuditByRefRequest) (*pkgAudit.ListAuditByRefResponse, error) {
	if m.FetchTransactionAuditTrailFunc == nil {
		return nil, nil
	}
	return m.FetchTransactionAuditTrailFunc(m.T, m, ctx, request)
}

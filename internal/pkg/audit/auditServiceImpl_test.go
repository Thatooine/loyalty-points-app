package audit

import (
	"context"
	"errors"
	"testing"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// TestServiceFetchTransactionAuditTrail_HappyPath proves the service forwards the
// scoped request to the repository and returns the entries it gets back.
func TestServiceFetchTransactionAuditTrail_HappyPath(t *testing.T) {
	var captured pkgAudit.ListAuditEntriesByTransactionRefRequest
	ref := "tx-1"
	repo := &MockAuditEntryRepository{
		T: t,
		ListByTransactionRefFunc: func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.ListAuditEntriesByTransactionRefRequest) (*pkgAudit.ListAuditEntriesByTransactionRefResponse, error) {
			captured = request
			return &pkgAudit.ListAuditEntriesByTransactionRefResponse{AuditEntries: []pkgAudit.AuditEntry{
				{ID: 1, UserID: "user-1", TransactionRef: &ref, Outcome: pkgAudit.OutcomeAccepted, Reason: "ok"},
			}}, nil
		},
	}
	service := NewAuditServiceImpl(repo)

	resp, err := service.FetchTransactionAuditTrail(context.Background(), pkgAudit.ListAuditByRefRequest{TransactionRef: ref, UserID: "user-1"})
	if err != nil {
		t.Fatalf("FetchTransactionAuditTrail() error = %v, want nil", err)
	}
	if len(resp.AuditEntries) != 1 || resp.AuditEntries[0].ID != 1 {
		t.Errorf("returned entries = %+v, want one entry with ID=1", resp.AuditEntries)
	}
	if captured.TransactionRef != ref || captured.UserID != "user-1" {
		t.Errorf("repo called with %+v, want TransactionRef=tx-1 UserID=user-1", captured)
	}
}

// TestServiceFetchTransactionAuditTrail_ValidationFailsClosed proves an invalid request
// is rejected before any persistence: the repository is never reached.
func TestServiceFetchTransactionAuditTrail_ValidationFailsClosed(t *testing.T) {
	repo := &MockAuditEntryRepository{
		T: t,
		ListByTransactionRefFunc: func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.ListAuditEntriesByTransactionRefRequest) (*pkgAudit.ListAuditEntriesByTransactionRefResponse, error) {
			t.Fatal("repository must not be called when the request is invalid")
			return nil, nil
		},
	}
	service := NewAuditServiceImpl(repo)

	_, err := service.FetchTransactionAuditTrail(context.Background(), pkgAudit.ListAuditByRefRequest{TransactionRef: "", UserID: "user-1"})
	if err == nil {
		t.Fatal("FetchTransactionAuditTrail() error = nil, want validation error")
	}
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Errorf("FetchTransactionAuditTrail() error = %v, want it to wrap %v", err, errs.ErrInvalidArgument)
	}
}

// TestServiceFetchTransactionAuditTrail_RepositoryError proves a repository failure
// surfaces and the underlying sentinel is preserved through the wrap.
func TestServiceFetchTransactionAuditTrail_RepositoryError(t *testing.T) {
	repo := &MockAuditEntryRepository{
		T: t,
		ListByTransactionRefFunc: func(t *testing.T, m *MockAuditEntryRepository, ctx context.Context, request pkgAudit.ListAuditEntriesByTransactionRefRequest) (*pkgAudit.ListAuditEntriesByTransactionRefResponse, error) {
			return nil, errs.ErrInternal
		},
	}
	service := NewAuditServiceImpl(repo)

	_, err := service.FetchTransactionAuditTrail(context.Background(), pkgAudit.ListAuditByRefRequest{TransactionRef: "tx-1", UserID: "user-1"})
	if !errors.Is(err, errs.ErrInternal) {
		t.Errorf("FetchTransactionAuditTrail() error = %v, want it to wrap %v", err, errs.ErrInternal)
	}
}

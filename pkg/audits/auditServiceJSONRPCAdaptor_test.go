package audits_test

import (
	"context"
	"net/http/httptest"
	"testing"

	internalaudit "github.com/Thatooine/loyalty-points-app/internal/pkg/audit"
	"github.com/Thatooine/loyalty-points-app/pkg/audits"
	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

// TestListByTransactionRef_HappyPath: with a valid claim, the adaptor scopes the
// listing to the caller's user id and maps the service entries onto the wire
// result.
func TestListByTransactionRef_HappyPath(t *testing.T) {
	const userID = "user-1"
	ref := "tx-1"
	mock := &internalaudit.MockAuditService{T: t}
	mock.ListByTransactionRefFunc = func(t *testing.T, m *internalaudit.MockAuditService, ctx context.Context, request audits.ListAuditByRefRequest) (*audits.ListAuditByRefResponse, error) {
		// The adaptor must pin the scope to the claim's user id, never the wire.
		if request.UserID != userID {
			t.Errorf("service called with UserID = %q, want %q", request.UserID, userID)
		}
		if request.TransactionRef != ref {
			t.Errorf("service called with TransactionRef = %q, want %q", request.TransactionRef, ref)
		}
		return &audits.ListAuditByRefResponse{AuditEntries: []audits.AuditEntry{
			{ID: 1, UserID: userID, TransactionRef: &ref, Outcome: audits.OutcomeAccepted, Reason: "ok"},
		}}, nil
	}

	adaptor := audits.NewAuditServiceJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil).WithContext(
		authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{
			UserID: userID,
			Role:   users.RoleMember,
		}),
	)

	var result audits.ListByTransactionRefResult
	if err := adaptor.ListByTransactionRef(req, &audits.ListByTransactionRefParams{TransactionRef: ref}, &result); err != nil {
		t.Fatalf("ListByTransactionRef returned error: %v", err)
	}

	if result.TransactionRef != ref {
		t.Errorf("result.TransactionRef = %q, want %q", result.TransactionRef, ref)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("len(result.Entries) = %d, want 1", len(result.Entries))
	}
	if result.Entries[0].ID != 1 || result.Entries[0].Outcome != "accepted" {
		t.Errorf("result.Entries[0] = %+v, want ID=1 Outcome=accepted", result.Entries[0])
	}
}

// TestListByTransactionRef_EmptyTrail: an empty slice from the service is a valid
// result (no entries the caller may see), surfaced as an empty Entries slice.
func TestListByTransactionRef_EmptyTrail(t *testing.T) {
	mock := &internalaudit.MockAuditService{T: t}
	mock.ListByTransactionRefFunc = func(t *testing.T, m *internalaudit.MockAuditService, ctx context.Context, request audits.ListAuditByRefRequest) (*audits.ListAuditByRefResponse, error) {
		return &audits.ListAuditByRefResponse{AuditEntries: nil}, nil
	}

	adaptor := audits.NewAuditServiceJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil).WithContext(
		authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{
			UserID: "user-1",
			Role:   users.RoleMember,
		}),
	)

	var result audits.ListByTransactionRefResult
	if err := adaptor.ListByTransactionRef(req, &audits.ListByTransactionRefParams{TransactionRef: "tx-1"}, &result); err != nil {
		t.Fatalf("ListByTransactionRef returned error: %v", err)
	}
	if result.Entries == nil {
		t.Error("result.Entries = nil, want non-nil empty slice")
	}
	if len(result.Entries) != 0 {
		t.Errorf("len(result.Entries) = %d, want 0", len(result.Entries))
	}
}

// TestListByTransactionRef_Unauthenticated: with no login claim on the context
// the adaptor fails closed and never touches the service.
func TestListByTransactionRef_Unauthenticated(t *testing.T) {
	mock := &internalaudit.MockAuditService{T: t}
	mock.ListByTransactionRefFunc = func(t *testing.T, m *internalaudit.MockAuditService, ctx context.Context, request audits.ListAuditByRefRequest) (*audits.ListAuditByRefResponse, error) {
		t.Fatal("service must not be called without a claim")
		return nil, nil
	}

	adaptor := audits.NewAuditServiceJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil) // no claim on context

	var result audits.ListByTransactionRefResult
	if err := adaptor.ListByTransactionRef(req, &audits.ListByTransactionRefParams{TransactionRef: "tx-1"}, &result); err == nil {
		t.Fatal("ListByTransactionRef: expected unauthorized error, got nil")
	}
}

// TestListByTransactionRef_ServiceError: a non-validation service error is mapped
// to the opaque internal message, never leaking internals.
func TestListByTransactionRef_ServiceError(t *testing.T) {
	mock := &internalaudit.MockAuditService{T: t}
	mock.ListByTransactionRefFunc = func(t *testing.T, m *internalaudit.MockAuditService, ctx context.Context, request audits.ListAuditByRefRequest) (*audits.ListAuditByRefResponse, error) {
		return nil, errs.ErrInternal
	}

	adaptor := audits.NewAuditServiceJSONRPCAdaptor(mock)
	req := httptest.NewRequest("POST", "/api", nil).WithContext(
		authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{
			UserID: "user-1",
			Role:   users.RoleMember,
		}),
	)

	var result audits.ListByTransactionRefResult
	err := adaptor.ListByTransactionRef(req, &audits.ListByTransactionRefParams{TransactionRef: "tx-1"}, &result)
	if err == nil {
		t.Fatal("ListByTransactionRef: expected an error, got nil")
	}
	if err.Error() != "could not retrieve audit trail" {
		t.Errorf("error = %q, want %q", err.Error(), "could not retrieve audit trail")
	}
}

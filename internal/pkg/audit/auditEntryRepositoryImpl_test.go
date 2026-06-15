package audit

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Thatooine/loyalty-points-app/internal/testsupport"
	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audit"
	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return testsupport.NewPostgresDB(t)
}

// ctxWithPerms returns a context carrying a login claim with the given
// permissions, mirroring what the authorization middleware places on a request.
func ctxWithPerms(perms ...string) context.Context {
	return authentication.ContextWithLoginClaim(
		context.Background(),
		authentication.LoginClaim{Permissions: perms},
	)
}

func createEntry(t *testing.T, repo *AuditEntryRepositoryImpl, owner, accountID, ref string) pkgAudit.AuditEntry {
	t.Helper()
	o, a, r := owner, accountID, ref
	resp, err := repo.Create(context.Background(), pkgAudit.CreateAuditEntryRequest{
		AuditEntry: pkgAudit.AuditEntry{
			TransactionRef: &r,
			AccountID:      &a,
			OwnerID:        &o,
			Outcome:        pkgAudit.OutcomeAccepted,
			Reason:         "ok",
			UserID:         owner,
			CreatedAt:      time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Create audit entry error = %v", err)
	}
	return resp.AuditEntry
}

func TestAuditEntryRepositoryImpl_OwnershipScopedByPermission(t *testing.T) {
	db := newTestDB(t)
	repo := NewAuditEntryRepositoryImpl(db)

	createEntry(t, repo, "owner-a", "acct-a", "tx-a")
	entryB := createEntry(t, repo, "owner-b", "acct-b", "tx-b")

	// Without audit:read:all the read is owner-scoped: owner-a sees only their
	// own entry and cannot read owner-b's, by list or by id.
	scoped := context.Background()
	got, err := repo.List(scoped, pkgAudit.ListAuditEntriesRequest{UserID: "owner-a"})
	if err != nil {
		t.Fatalf("scoped List error = %v", err)
	}
	if len(got.AuditEntries) != 1 {
		t.Fatalf("scoped List returned %d entries, want 1", len(got.AuditEntries))
	}
	if _, err := repo.GetByID(scoped, pkgAudit.GetAuditEntryByIDRequest{ID: entryB.ID, UserID: "owner-a"}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("scoped cross-owner GetByID error = %v, want errs.ErrNotFound", err)
	}
	if byAcct, err := repo.ListByAccountID(scoped, pkgAudit.ListAuditEntriesByAccountIDRequest{AccountID: "acct-b", UserID: "owner-a"}); err != nil {
		t.Fatalf("scoped ListByAccountID error = %v", err)
	} else if len(byAcct.AuditEntries) != 0 {
		t.Fatalf("scoped ListByAccountID returned %d entries, want 0", len(byAcct.AuditEntries))
	}

	// With audit:read:all the owner filter is dropped across every read path.
	all := ctxWithPerms(authorization.PermAuditReadAll)
	got, err = repo.List(all, pkgAudit.ListAuditEntriesRequest{UserID: "owner-a"})
	if err != nil {
		t.Fatalf("all-scope List error = %v", err)
	}
	if len(got.AuditEntries) != 2 {
		t.Fatalf("all-scope List returned %d entries, want 2", len(got.AuditEntries))
	}
	if _, err := repo.GetByID(all, pkgAudit.GetAuditEntryByIDRequest{ID: entryB.ID, UserID: "owner-a"}); err != nil {
		t.Fatalf("all-scope GetByID error = %v", err)
	}
	if byAcct, err := repo.ListByAccountID(all, pkgAudit.ListAuditEntriesByAccountIDRequest{AccountID: "acct-b", UserID: "owner-a"}); err != nil {
		t.Fatalf("all-scope ListByAccountID error = %v", err)
	} else if len(byAcct.AuditEntries) != 1 {
		t.Fatalf("all-scope ListByAccountID returned %d entries, want 1", len(byAcct.AuditEntries))
	}
}

package wallet

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	internalAudit "github.com/Thatooine/loyalty-points-app/internal/pkg/audit"
	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audit"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/sqlite"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
)

func newWalletService(db *sql.DB) (*WalletServiceImpl, pkgAudit.AuditEntryRepository) {
	auditRepo := internalAudit.NewAuditEntryRepositoryImpl(db)
	service := NewWalletServiceImpl(
		sqlite.NewSQLiteTxManager(db),
		internalAccounts.NewAccountRepositoryImpl(db),
		NewTransactionRepositoryImpl(db),
		auditRepo,
	)
	return service, auditRepo
}

func processRequest(ref, accountID string, kind pkgWallet.Kind, points int64) pkgWallet.ProcessTransactionRequest {
	return pkgWallet.ProcessTransactionRequest{
		Ref:        ref,
		AccountID:  accountID,
		Kind:       kind,
		Points:     points,
		OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		Actor:      "user-" + accountID,
		Source:     "api",
	}
}

// auditOutcomes returns the count of audit rows per outcome.
func auditOutcomes(t *testing.T, repo pkgAudit.AuditEntryRepository) map[pkgAudit.Outcome]int {
	t.Helper()
	resp, err := repo.List(context.Background(), pkgAudit.ListAuditEntriesRequest{})
	if err != nil {
		t.Fatalf("audit List() error = %v", err)
	}
	counts := map[pkgAudit.Outcome]int{}
	for _, entry := range resp.AuditEntries {
		counts[entry.Outcome]++
	}
	return counts
}

// ledgerSum returns SUM(points) over the ledger for an account — the invariant
// that must always equal the materialised balance.
func ledgerSum(t *testing.T, db *sql.DB, accountID string) int64 {
	t.Helper()
	var sum sql.NullInt64
	if err := db.QueryRow(`SELECT SUM(points) FROM transactions WHERE account_id = ?`, accountID).Scan(&sum); err != nil {
		t.Fatalf("ledger sum query error = %v", err)
	}
	return sum.Int64
}

func TestProcessTransaction_Earn(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, auditRepo := newWalletService(db)

	resp, err := service.ProcessTransaction(ctx, processRequest("tx-001", "member-123", pkgWallet.KindEarn, 150))
	if err != nil {
		t.Fatalf("ProcessTransaction() error = %v", err)
	}
	if resp.Duplicate {
		t.Fatalf("Duplicate = true, want false")
	}
	if resp.Balance != 150 {
		t.Fatalf("Balance = %d, want 150", resp.Balance)
	}
	if resp.Transaction.Points != 150 {
		t.Fatalf("Transaction.Points = %d, want 150", resp.Transaction.Points)
	}
	if got := auditOutcomes(t, auditRepo); got[pkgAudit.OutcomeAccepted] != 1 {
		t.Fatalf("accepted audit rows = %d, want 1", got[pkgAudit.OutcomeAccepted])
	}
}

func TestProcessTransaction_Spend(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, _ := newWalletService(db)

	if _, err := service.ProcessTransaction(ctx, processRequest("tx-001", "member-123", pkgWallet.KindEarn, 150)); err != nil {
		t.Fatalf("earn error = %v", err)
	}

	resp, err := service.ProcessTransaction(ctx, processRequest("tx-002", "member-123", pkgWallet.KindSpend, 50))
	if err != nil {
		t.Fatalf("spend error = %v", err)
	}
	if resp.Balance != 100 {
		t.Fatalf("Balance = %d, want 100", resp.Balance)
	}
	if resp.Transaction.Points != -50 {
		t.Fatalf("spend Transaction.Points = %d, want -50", resp.Transaction.Points)
	}

	// invariant: materialised balance equals the ledger sum
	if sum := ledgerSum(t, db, "member-123"); sum != resp.Balance {
		t.Fatalf("ledger sum = %d, balance = %d (must be equal)", sum, resp.Balance)
	}
}

func TestProcessTransaction_DuplicateRef(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, auditRepo := newWalletService(db)

	first, err := service.ProcessTransaction(ctx, processRequest("tx-001", "member-123", pkgWallet.KindEarn, 150))
	if err != nil {
		t.Fatalf("first earn error = %v", err)
	}

	// resubmit the same ref — must not double count
	second, err := service.ProcessTransaction(ctx, processRequest("tx-001", "member-123", pkgWallet.KindEarn, 150))
	if err != nil {
		t.Fatalf("duplicate earn error = %v", err)
	}
	if !second.Duplicate {
		t.Fatalf("Duplicate = false, want true")
	}
	if second.Balance != first.Balance {
		t.Fatalf("balance changed on duplicate: %d -> %d", first.Balance, second.Balance)
	}
	if second.Transaction.Ref != "tx-001" {
		t.Fatalf("duplicate returned ref %q, want tx-001", second.Transaction.Ref)
	}

	counts := auditOutcomes(t, auditRepo)
	if counts[pkgAudit.OutcomeAccepted] != 1 || counts[pkgAudit.OutcomeDuplicate] != 1 {
		t.Fatalf("audit outcomes = %+v, want 1 accepted + 1 duplicate", counts)
	}
}

func TestProcessTransaction_Overdraft(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, auditRepo := newWalletService(db)

	if _, err := service.ProcessTransaction(ctx, processRequest("tx-001", "member-123", pkgWallet.KindEarn, 100)); err != nil {
		t.Fatalf("earn error = %v", err)
	}

	_, err := service.ProcessTransaction(ctx, processRequest("tx-002", "member-123", pkgWallet.KindSpend, 150))
	if !errors.Is(err, errs.ErrInsufficientBalance) {
		t.Fatalf("overdraft error = %v, want errs.ErrInsufficientBalance", err)
	}

	// balance unchanged, and the rejected ledger insert rolled back so the
	// ref is free again
	txnRepo := NewTransactionRepositoryImpl(db)
	if _, err := txnRepo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-002"}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("rejected ledger row survived: error = %v, want errs.ErrNotFound", err)
	}
	if sum := ledgerSum(t, db, "member-123"); sum != 100 {
		t.Fatalf("ledger sum after rejected spend = %d, want 100", sum)
	}

	counts := auditOutcomes(t, auditRepo)
	if counts[pkgAudit.OutcomeRejected] != 1 {
		t.Fatalf("rejected audit rows = %d, want 1", counts[pkgAudit.OutcomeRejected])
	}
}

func TestProcessTransaction_Adjust(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, _ := newWalletService(db)

	if _, err := service.ProcessTransaction(ctx, processRequest("tx-001", "member-123", pkgWallet.KindEarn, 100)); err != nil {
		t.Fatalf("earn error = %v", err)
	}

	// negative adjustment
	down, err := service.ProcessTransaction(ctx, processRequest("adj-001", "member-123", pkgWallet.KindAdjust, -30))
	if err != nil {
		t.Fatalf("negative adjust error = %v", err)
	}
	if down.Balance != 70 || down.Transaction.Points != -30 {
		t.Fatalf("after -30 adjust: balance=%d points=%d, want 70 / -30", down.Balance, down.Transaction.Points)
	}

	// positive adjustment
	up, err := service.ProcessTransaction(ctx, processRequest("adj-002", "member-123", pkgWallet.KindAdjust, 50))
	if err != nil {
		t.Fatalf("positive adjust error = %v", err)
	}
	if up.Balance != 120 || up.Transaction.Points != 50 {
		t.Fatalf("after +50 adjust: balance=%d points=%d, want 120 / 50", up.Balance, up.Transaction.Points)
	}
}

func TestProcessTransaction_UnknownAccount(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	service, auditRepo := newWalletService(db)

	_, err := service.ProcessTransaction(ctx, processRequest("tx-001", "ghost", pkgWallet.KindEarn, 100))
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("unknown account error = %v, want errs.ErrNotFound", err)
	}

	if counts := auditOutcomes(t, auditRepo); counts[pkgAudit.OutcomeRejected] != 1 {
		t.Fatalf("rejected audit rows = %d, want 1", counts[pkgAudit.OutcomeRejected])
	}
}

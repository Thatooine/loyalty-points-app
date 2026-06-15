package wallet

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	internalAudit "github.com/Thatooine/loyalty-points-app/internal/pkg/audit"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audit"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
)

func newWalletService(db *sql.DB) (*WalletServiceImpl, pkgAudit.AuditEntryRepository) {
	auditRepo := internalAudit.NewAuditEntryRepositoryImpl(db)
	service := NewWalletServiceImpl(
		postgres.NewPostgresTxManager(db),
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
		UserID:     "user-" + accountID,
	}
}

// auditOutcomes returns the count of audit rows per outcome. It reads the table
// directly rather than through the repository's List, which is now ownership-
// scoped and so cannot return rejected rows for an unknown account (nil owner).
func auditOutcomes(t *testing.T, db *sql.DB) map[pkgAudit.Outcome]int {
	t.Helper()
	rows, err := db.Query(`SELECT outcome, COUNT(*) FROM audit_log GROUP BY outcome`)
	if err != nil {
		t.Fatalf("audit outcomes query error = %v", err)
	}
	defer rows.Close()
	counts := map[pkgAudit.Outcome]int{}
	for rows.Next() {
		var outcome string
		var n int
		if err := rows.Scan(&outcome, &n); err != nil {
			t.Fatalf("scan audit outcome error = %v", err)
		}
		counts[pkgAudit.Outcome(outcome)] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit outcomes error = %v", err)
	}
	return counts
}

// ledgerSum returns SUM(points) over the ledger for an account — the invariant
// that must always equal the materialised balance.
func ledgerSum(t *testing.T, db *sql.DB, accountID string) int64 {
	t.Helper()
	var sum sql.NullInt64
	if err := db.QueryRow(`SELECT SUM(points) FROM transactions WHERE account_id = $1`, accountID).Scan(&sum); err != nil {
		t.Fatalf("ledger sum query error = %v", err)
	}
	return sum.Int64
}

// accountBalance returns the materialised account balance — which includes any
// seeded credit, unlike ledgerSum (which only sums recorded transaction rows).
func accountBalance(t *testing.T, db *sql.DB, accountID string) int64 {
	t.Helper()
	var balance int64
	if err := db.QueryRow(`SELECT balance FROM accounts WHERE id = $1`, accountID).Scan(&balance); err != nil {
		t.Fatalf("account balance query error = %v", err)
	}
	return balance
}

func TestProcessTransaction_Earn(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, _ := newWalletService(db)

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
	if got := auditOutcomes(t, db); got[pkgAudit.OutcomeAccepted] != 1 {
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
	service, _ := newWalletService(db)

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

	counts := auditOutcomes(t, db)
	if counts[pkgAudit.OutcomeAccepted] != 1 || counts[pkgAudit.OutcomeDuplicate] != 1 {
		t.Fatalf("audit outcomes = %+v, want 1 accepted + 1 duplicate", counts)
	}
}

func TestProcessTransaction_Overdraft(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, _ := newWalletService(db)

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
	if _, err := txnRepo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-002", UserID: "user-member-123"}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("rejected ledger row survived: error = %v, want errs.ErrNotFound", err)
	}
	if sum := ledgerSum(t, db, "member-123"); sum != 100 {
		t.Fatalf("ledger sum after rejected spend = %d, want 100", sum)
	}

	counts := auditOutcomes(t, db)
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

func TestProcessTransaction_NotOwnerRejected(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123") // owned by user-member-123
	service, _ := newWalletService(db)

	// a non-admin user who does not own the account: the ownership-scoped read
	// makes it indistinguishable from a missing account.
	req := processRequest("tx-001", "member-123", pkgWallet.KindEarn, 100)
	req.UserID = "user-intruder"

	_, err := service.ProcessTransaction(ctx, req)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("error = %v, want errs.ErrNotFound", err)
	}

	// nothing was applied and the ledger stayed empty
	if sum := ledgerSum(t, db, "member-123"); sum != 0 {
		t.Fatalf("ledger sum = %d, want 0 (nothing applied)", sum)
	}
	if counts := auditOutcomes(t, db); counts[pkgAudit.OutcomeRejected] != 1 {
		t.Fatalf("rejected audit rows = %d, want 1", counts[pkgAudit.OutcomeRejected])
	}
}

func TestProcessTransactionBatch_OrderedApplication(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	// seed a starting balance of 10
	repo := internalAccounts.NewAccountRepositoryImpl(db)
	if _, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "member-123", Delta: 10, UserID: "user-member-123"}); err != nil {
		t.Fatalf("seed credit error = %v", err)
	}
	service, _ := newWalletService(db)

	// earn 10 then spend 20: applied in this order the spend is funded (10+10-20=0)
	batch := pkgWallet.ProcessTransactionBatchRequest{
		Transactions: []pkgWallet.ProcessTransactionRequest{
			processRequest("tx-earn", "member-123", pkgWallet.KindEarn, 10),
			processRequest("tx-spend", "member-123", pkgWallet.KindSpend, 20),
		},
	}

	resp, err := service.ProcessTransactionBatch(ctx, batch)
	if err != nil {
		t.Fatalf("ProcessTransactionBatch() error = %v", err)
	}
	if resp.Accepted != 2 || resp.Rejected != 0 {
		t.Fatalf("tallies: accepted=%d rejected=%d, want 2/0", resp.Accepted, resp.Rejected)
	}
	if resp.Results[0].Outcome != pkgWallet.BatchOutcomeAccepted || resp.Results[1].Outcome != pkgWallet.BatchOutcomeAccepted {
		t.Fatalf("outcomes = %q/%q, want accepted/accepted", resp.Results[0].Outcome, resp.Results[1].Outcome)
	}
	// running balances: 10 +10 = 20, then -20 = 0
	if resp.Results[0].Balance != 20 || resp.Results[1].Balance != 0 {
		t.Fatalf("balances = %d/%d, want 20/0", resp.Results[0].Balance, resp.Results[1].Balance)
	}
	if got := accountBalance(t, db, "member-123"); got != 0 {
		t.Fatalf("final balance = %d, want 0", got)
	}
}

func TestProcessTransactionBatch_WrongOrderRejectsSpend(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	repo := internalAccounts.NewAccountRepositoryImpl(db)
	if _, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "member-123", Delta: 10, UserID: "user-member-123"}); err != nil {
		t.Fatalf("seed credit error = %v", err)
	}
	service, _ := newWalletService(db)

	// spend 20 BEFORE the earn that funds it: the floor rejects the spend, but
	// the batch continues and the earn still applies. This is the order-
	// dependence the CLI's OccurredAt sort exists to avoid.
	batch := pkgWallet.ProcessTransactionBatchRequest{
		Transactions: []pkgWallet.ProcessTransactionRequest{
			processRequest("tx-spend", "member-123", pkgWallet.KindSpend, 20),
			processRequest("tx-earn", "member-123", pkgWallet.KindEarn, 10),
		},
	}

	resp, err := service.ProcessTransactionBatch(ctx, batch)
	if err != nil {
		t.Fatalf("ProcessTransactionBatch() error = %v", err)
	}
	if resp.Accepted != 1 || resp.Rejected != 1 {
		t.Fatalf("tallies: accepted=%d rejected=%d, want 1/1", resp.Accepted, resp.Rejected)
	}
	if resp.Results[0].Outcome != pkgWallet.BatchOutcomeRejected {
		t.Fatalf("first outcome = %q, want rejected", resp.Results[0].Outcome)
	}
	if resp.Results[0].Reason != "insufficient balance" {
		t.Fatalf("rejection reason = %q, want \"insufficient balance\"", resp.Results[0].Reason)
	}
	// only the earn applied: 10 + 10 = 20
	if got := accountBalance(t, db, "member-123"); got != 20 {
		t.Fatalf("final balance = %d, want 20", got)
	}
}

func TestProcessTransactionBatch_DuplicateWithinBatch(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	service, _ := newWalletService(db)

	batch := pkgWallet.ProcessTransactionBatchRequest{
		Transactions: []pkgWallet.ProcessTransactionRequest{
			processRequest("tx-1", "member-123", pkgWallet.KindEarn, 100),
			processRequest("tx-1", "member-123", pkgWallet.KindEarn, 100), // same ref
		},
	}

	resp, err := service.ProcessTransactionBatch(ctx, batch)
	if err != nil {
		t.Fatalf("ProcessTransactionBatch() error = %v", err)
	}
	if resp.Accepted != 1 || resp.Duplicate != 1 {
		t.Fatalf("tallies: accepted=%d duplicate=%d, want 1/1", resp.Accepted, resp.Duplicate)
	}
	// the duplicate never double-counted
	if got := ledgerSum(t, db, "member-123"); got != 100 {
		t.Fatalf("final balance = %d, want 100", got)
	}
}

func TestProcessTransaction_UnknownAccount(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	service, _ := newWalletService(db)

	_, err := service.ProcessTransaction(ctx, processRequest("tx-001", "ghost", pkgWallet.KindEarn, 100))
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("unknown account error = %v, want errs.ErrNotFound", err)
	}

	if counts := auditOutcomes(t, db); counts[pkgAudit.OutcomeRejected] != 1 {
		t.Fatalf("rejected audit rows = %d, want 1", counts[pkgAudit.OutcomeRejected])
	}
}

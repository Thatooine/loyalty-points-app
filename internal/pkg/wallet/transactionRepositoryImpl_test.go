package wallet

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
	"github.com/Thatooine/loyalty-points-app/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=foreign_keys(1)"
	db, err := sqlite.NewClient(ctx, dsn)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return db
}

func createTestAccount(t *testing.T, db *sql.DB, accountID string) {
	t.Helper()
	accountRepo := internalAccounts.NewAccountRepositoryImpl(db)
	_, err := accountRepo.Create(context.Background(), pkgAccounts.CreateAccountRequest{
		Account: pkgAccounts.Account{
			AccountID:    accountID,
			Name:         "Test Member",
			Role:         pkgAccounts.RoleMember,
			PasswordHash: "bcrypt-hash",
			CreatedAt:    time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Create account error = %v", err)
	}
}

func testTransaction(ref, accountID string) pkgWallet.Transaction {
	return pkgWallet.Transaction{
		Ref:        ref,
		AccountID:  accountID,
		Kind:       pkgWallet.KindEarn,
		Points:     150,
		OccurredAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		RecordedAt: time.Date(2026, 6, 1, 10, 0, 1, 0, time.UTC),
		CreatedBy:  accountID,
	}
}

func TestTransactionRepositoryImpl_CreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	repo := NewTransactionRepositoryImpl(db)

	want := testTransaction("tx-001", "member-123")
	if _, err := repo.Create(ctx, pkgWallet.CreateTransactionRequest{Transaction: want}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-001"})
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Transaction != want {
		t.Fatalf("GetByID() = %+v, want %+v", got.Transaction, want)
	}
}

func TestTransactionRepositoryImpl_DuplicateRef(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestAccount(t, db, "member-123")
	repo := NewTransactionRepositoryImpl(db)

	transaction := testTransaction("tx-001", "member-123")
	if _, err := repo.Create(ctx, pkgWallet.CreateTransactionRequest{Transaction: transaction}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := repo.Create(ctx, pkgWallet.CreateTransactionRequest{Transaction: transaction})
	if !errors.Is(err, errs.ErrDuplicateRef) {
		t.Fatalf("Create() duplicate error = %v, want errs.ErrDuplicateRef", err)
	}
}

func TestTransactionRepositoryImpl_GetByIDNotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewTransactionRepositoryImpl(db)

	_, err := repo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "missing"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want errs.ErrNotFound", err)
	}
}

// TestRunInTx_AtomicAcrossRepositories proves transaction awareness: an
// account insert and a transaction insert in one unit of work both roll back
// when the unit of work fails.
func TestRunInTx_AtomicAcrossRepositories(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	txManager := sqlite.NewTxManager(db)
	accountRepo := internalAccounts.NewAccountRepositoryImpl(db)
	transactionRepo := NewTransactionRepositoryImpl(db)

	failure := errors.New("forced failure")
	err := txManager.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := accountRepo.Create(ctx, pkgAccounts.CreateAccountRequest{
			Account: pkgAccounts.Account{
				AccountID:    "member-123",
				Name:         "Test Member",
				Role:         pkgAccounts.RoleMember,
				PasswordHash: "bcrypt-hash",
				CreatedAt:    time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
			},
		}); err != nil {
			return err
		}
		if _, err := transactionRepo.Create(ctx, pkgWallet.CreateTransactionRequest{
			Transaction: testTransaction("tx-001", "member-123"),
		}); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("RunInTx() error = %v, want forced failure", err)
	}

	if _, err := accountRepo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123"}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("account survived rollback: error = %v, want errs.ErrNotFound", err)
	}
	if _, err := transactionRepo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-001"}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("transaction survived rollback: error = %v, want errs.ErrNotFound", err)
	}
}

// TestRunInTx_CommitAcrossRepositories proves the happy path: both writes in
// the unit of work land together.
func TestRunInTx_CommitAcrossRepositories(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	txManager := sqlite.NewTxManager(db)
	accountRepo := internalAccounts.NewAccountRepositoryImpl(db)
	transactionRepo := NewTransactionRepositoryImpl(db)

	err := txManager.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := accountRepo.Create(ctx, pkgAccounts.CreateAccountRequest{
			Account: pkgAccounts.Account{
				AccountID:    "member-123",
				Name:         "Test Member",
				Role:         pkgAccounts.RoleMember,
				PasswordHash: "bcrypt-hash",
				CreatedAt:    time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
			},
		}); err != nil {
			return err
		}
		_, err := transactionRepo.Create(ctx, pkgWallet.CreateTransactionRequest{
			Transaction: testTransaction("tx-001", "member-123"),
		})
		return err
	})
	if err != nil {
		t.Fatalf("RunInTx() error = %v", err)
	}

	if _, err := accountRepo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123"}); err != nil {
		t.Fatalf("account not committed: %v", err)
	}
	if _, err := transactionRepo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-001"}); err != nil {
		t.Fatalf("transaction not committed: %v", err)
	}
}

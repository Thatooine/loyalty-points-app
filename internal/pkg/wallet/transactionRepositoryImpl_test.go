package wallet

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	"github.com/Thatooine/loyalty-points-app/internal/testsupport"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return testsupport.NewPostgresDB(t)
}

func createTestUser(t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	userRepo := internalUsers.NewUserRepositoryImpl(db)
	_, err := userRepo.Create(context.Background(), pkgUsers.CreateUserRequest{
		User: pkgUsers.User{
			ID:           userID,
			Email:        userID + "@example.com",
			PasswordHash: "bcrypt-hash",
			Role:         pkgUsers.RoleMember,
			CreatedAt:    time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Create user error = %v", err)
	}
}

func createTestAccount(t *testing.T, db *sql.DB, accountID string) {
	t.Helper()
	createTestUser(t, db, "user-"+accountID)
	accountRepo := internalAccounts.NewAccountRepositoryImpl(db)
	_, err := accountRepo.Create(context.Background(), pkgAccounts.CreateAccountRequest{
		Account: pkgAccounts.Account{
			ID:        accountID,
			OwnerID:   "user-" + accountID,
			Name:      "Test Member",
			CreatedAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
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

	created, err := repo.Create(ctx, pkgWallet.CreateTransactionRequest{Transaction: testTransaction("tx-001", "member-123")})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// Create assigns a UUID, so compare the round-trip against what was stored.
	if created.Transaction.ID == "" {
		t.Fatalf("Create() did not assign an ID")
	}

	got, err := repo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-001"})
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Transaction != created.Transaction {
		t.Fatalf("GetByID() = %+v, want %+v", got.Transaction, created.Transaction)
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
	txManager := postgres.NewPostgresTxManager(db)
	accountRepo := internalAccounts.NewAccountRepositoryImpl(db)
	transactionRepo := NewTransactionRepositoryImpl(db)
	createTestUser(t, db, "user-1")

	failure := errors.New("forced failure")
	err := txManager.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := accountRepo.Create(ctx, pkgAccounts.CreateAccountRequest{
			Account: pkgAccounts.Account{
				ID:        "member-123",
				OwnerID:   "user-1",
				Name:      "Test Member",
				CreatedAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
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
	txManager := postgres.NewPostgresTxManager(db)
	accountRepo := internalAccounts.NewAccountRepositoryImpl(db)
	transactionRepo := NewTransactionRepositoryImpl(db)
	createTestUser(t, db, "user-1")

	err := txManager.RunInTx(ctx, func(ctx context.Context) error {
		if _, err := accountRepo.Create(ctx, pkgAccounts.CreateAccountRequest{
			Account: pkgAccounts.Account{
				ID:        "member-123",
				OwnerID:   "user-1",
				Name:      "Test Member",
				CreatedAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
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

package accounts

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	"github.com/Thatooine/loyalty-points-app/internal/testsupport"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
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

func testAccount(accountID, userID string) pkgAccounts.Account {
	return pkgAccounts.Account{
		ID:        accountID,
		OwnerID:   userID,
		Name:      "Test Member",
		Balance:   0,
		CreatedAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

func TestAccountRepositoryImpl_CreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestUser(t, db, "user-1")
	repo := NewAccountRepositoryImpl(db)

	want := testAccount("member-123", "user-1")
	if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: want}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123"})
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Account != want {
		t.Fatalf("GetByID() = %+v, want %+v", got.Account, want)
	}
}

func TestAccountRepositoryImpl_CreateDuplicate(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestUser(t, db, "user-1")
	repo := NewAccountRepositoryImpl(db)

	account := testAccount("member-123", "user-1")
	if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: account}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: account})
	if !errors.Is(err, errs.ErrAlreadyExists) {
		t.Fatalf("Create() duplicate error = %v, want errs.ErrAlreadyExists", err)
	}
}

func TestAccountRepositoryImpl_GetByIDNotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewAccountRepositoryImpl(db)

	_, err := repo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "missing"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want errs.ErrNotFound", err)
	}
}

func TestAccountRepositoryImpl_GetByIDOwnershipScoped(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestUser(t, db, "owner")
	createTestUser(t, db, "intruder")
	repo := NewAccountRepositoryImpl(db)

	if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: testAccount("member-123", "owner")}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// the owner can read it
	if _, err := repo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123", UserID: "owner"}); err != nil {
		t.Fatalf("owner GetByID() error = %v", err)
	}

	// a non-owner gets ErrNotFound (indistinguishable from missing)
	_, err := repo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123", UserID: "intruder"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("non-owner GetByID() error = %v, want errs.ErrNotFound", err)
	}

	// an unscoped lookup (no UserID) still returns it — internal/admin path
	if _, err := repo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123"}); err != nil {
		t.Fatalf("unscoped GetByID() error = %v", err)
	}
}

func TestAccountRepositoryImpl_GetAccountBalance(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestUser(t, db, "owner")
	createTestUser(t, db, "intruder")
	repo := NewAccountRepositoryImpl(db)

	if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: testAccount("member-123", "owner")}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "member-123", Delta: 250}); err != nil {
		t.Fatalf("seed credit error = %v", err)
	}

	// owner reads the balance
	got, err := repo.GetAccountBalance(ctx, pkgAccounts.GetAccountBalanceRequest{AccountID: "member-123", UserID: "owner"})
	if err != nil {
		t.Fatalf("GetAccountBalance() error = %v", err)
	}
	if got.Balance != 250 {
		t.Fatalf("Balance = %d, want 250", got.Balance)
	}

	// non-owner is denied via ErrNotFound
	_, err = repo.GetAccountBalance(ctx, pkgAccounts.GetAccountBalanceRequest{AccountID: "member-123", UserID: "intruder"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("non-owner GetAccountBalance() error = %v, want errs.ErrNotFound", err)
	}

	// missing account
	_, err = repo.GetAccountBalance(ctx, pkgAccounts.GetAccountBalanceRequest{AccountID: "missing", UserID: "owner"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("missing GetAccountBalance() error = %v, want errs.ErrNotFound", err)
	}
}

func TestAccountRepositoryImpl_UpdateAccountBalance(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestUser(t, db, "user-1")
	repo := NewAccountRepositoryImpl(db)

	if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: testAccount("member-123", "user-1")}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// credit
	credit, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "member-123", Delta: 150})
	if err != nil {
		t.Fatalf("UpdateAccountBalance() credit error = %v", err)
	}
	if credit.Balance != 150 {
		t.Fatalf("after credit balance = %d, want 150", credit.Balance)
	}

	// debit within balance
	debit, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "member-123", Delta: -50})
	if err != nil {
		t.Fatalf("UpdateAccountBalance() debit error = %v", err)
	}
	if debit.Balance != 100 {
		t.Fatalf("after debit balance = %d, want 100", debit.Balance)
	}
}

func TestAccountRepositoryImpl_UpdateAccountBalanceOverdraft(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestUser(t, db, "user-1")
	repo := NewAccountRepositoryImpl(db)

	if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: testAccount("member-123", "user-1")}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "member-123", Delta: 100}); err != nil {
		t.Fatalf("seed credit error = %v", err)
	}

	_, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "member-123", Delta: -150})
	if !errors.Is(err, errs.ErrInsufficientBalance) {
		t.Fatalf("overdraft error = %v, want errs.ErrInsufficientBalance", err)
	}

	// balance must be unchanged
	got, err := repo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123"})
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Account.Balance != 100 {
		t.Fatalf("balance after rejected overdraft = %d, want 100", got.Account.Balance)
	}
}

func TestAccountRepositoryImpl_UpdateAccountBalanceUnknownAccount(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewAccountRepositoryImpl(db)

	_, err := repo.UpdateAccountBalance(ctx, pkgAccounts.UpdateAccountBalanceRequest{AccountID: "missing", Delta: 100})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("unknown account error = %v, want errs.ErrNotFound", err)
	}
}

func TestAccountRepositoryImpl_List(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	createTestUser(t, db, "user-1")
	repo := NewAccountRepositoryImpl(db)

	// one user holding multiple accounts
	for _, accountID := range []string{"member-1", "member-2"} {
		if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: testAccount(accountID, "user-1")}); err != nil {
			t.Fatalf("Create(%s) error = %v", accountID, err)
		}
	}

	got, err := repo.List(ctx, pkgAccounts.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got.Accounts) != 2 {
		t.Fatalf("List() returned %d accounts, want 2", len(got.Accounts))
	}
}

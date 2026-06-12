package accounts

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/sqlite"
)

func newTestRepository(t *testing.T) *AccountRepositoryImpl {
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

	return NewAccountRepositoryImpl(db)
}

func testAccount(accountID string) pkgAccounts.Account {
	return pkgAccounts.Account{
		AccountID:    accountID,
		Name:         "Test Member",
		Role:         pkgAccounts.RoleMember,
		PasswordHash: "bcrypt-hash",
		Balance:      0,
		CreatedAt:    time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

func TestAccountRepositoryImpl_CreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	want := testAccount("member-123")
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
	repo := newTestRepository(t)

	account := testAccount("member-123")
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
	repo := newTestRepository(t)

	_, err := repo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "missing"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want errs.ErrNotFound", err)
	}
}

func TestAccountRepositoryImpl_List(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	for _, accountID := range []string{"member-1", "member-2"} {
		if _, err := repo.Create(ctx, pkgAccounts.CreateAccountRequest{Account: testAccount(accountID)}); err != nil {
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

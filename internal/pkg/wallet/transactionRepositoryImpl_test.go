package wallet

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	"github.com/Thatooine/loyalty-points-app/internal/testsupport"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
	pkgWallet "github.com/Thatooine/loyalty-points-app/pkg/wallet"
)

// ctxWithPerms returns a context carrying a login claim with the given
// permissions, mirroring what the authorization middleware places on a request.
func ctxWithPerms(perms ...string) context.Context {
	return authentication.ContextWithLoginClaim(
		context.Background(),
		authentication.LoginClaim{Permissions: perms},
	)
}

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
		OwnerID:    "user-" + accountID,
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

	got, err := repo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-001", UserID: "user-member-123"})
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

func TestTransactionRepositoryImpl_AllScopeReadsAcrossOwners(t *testing.T) {
	db := newTestDB(t)
	createTestAccount(t, db, "acct-a")
	createTestAccount(t, db, "acct-b")
	repo := NewTransactionRepositoryImpl(db)

	seed := context.Background()
	if _, err := repo.Create(seed, pkgWallet.CreateTransactionRequest{Transaction: testTransaction("tx-a", "acct-a")}); err != nil {
		t.Fatalf("Create(tx-a) error = %v", err)
	}
	if _, err := repo.Create(seed, pkgWallet.CreateTransactionRequest{Transaction: testTransaction("tx-b", "acct-b")}); err != nil {
		t.Fatalf("Create(tx-b) error = %v", err)
	}

	// Without transaction:read:all, a non-owner gets ErrNotFound.
	if _, err := repo.GetByID(seed, pkgWallet.GetTransactionByIDRequest{Ref: "tx-b", UserID: "user-acct-a"}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("scoped non-owner GetByID error = %v, want errs.ErrNotFound", err)
	}

	// With transaction:read:all the owner filter is dropped, so the same caller
	// reads another owner's transaction and lists across owners.
	ctx := ctxWithPerms(authorization.PermTransactionReadAll)
	if _, err := repo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-b", UserID: "user-acct-a"}); err != nil {
		t.Fatalf("all-scope GetByID error = %v", err)
	}
	got, err := repo.List(ctx, pkgWallet.ListTransactionsRequest{UserID: "user-acct-a"})
	if err != nil {
		t.Fatalf("all-scope List error = %v", err)
	}
	if len(got.Transactions) != 2 {
		t.Fatalf("all-scope List returned %d transactions, want 2", len(got.Transactions))
	}
}

// seedTransaction creates one transaction stamped with a specific recorded_at,
// so pagination tests can control the (recorded_at DESC, ref) ordering.
func seedTransaction(t *testing.T, repo *TransactionRepositoryImpl, ref, accountID string, recordedAt time.Time) {
	t.Helper()
	tx := testTransaction(ref, accountID)
	tx.RecordedAt = recordedAt
	if _, err := repo.Create(context.Background(), pkgWallet.CreateTransactionRequest{Transaction: tx}); err != nil {
		t.Fatalf("seed transaction %s error = %v", ref, err)
	}
}

// TestTransactionRepositoryImpl_ListPagination walks every page via the cursor
// and proves the keyset covers the whole set exactly once, newest-first, with no
// page exceeding PageSize and the final page reporting no NextCursor.
func TestTransactionRepositoryImpl_ListPagination(t *testing.T) {
	db := newTestDB(t)
	createTestAccount(t, db, "page-acct")
	repo := NewTransactionRepositoryImpl(db)

	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	// Seed newest-last so the expected newest-first order is tx-4..tx-0.
	for i := 0; i < 5; i++ {
		seedTransaction(t, repo, fmt.Sprintf("tx-%d", i), "page-acct", base.Add(time.Duration(i)*time.Second))
	}

	ctx := context.Background()
	const pageSize = 2
	var got []string
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatalf("pagination did not terminate after %d pages", pages)
		}
		resp, err := repo.List(ctx, pkgWallet.ListTransactionsRequest{UserID: "user-page-acct", PageSize: pageSize, Cursor: cursor})
		if err != nil {
			t.Fatalf("List(cursor=%q) error = %v", cursor, err)
		}
		if len(resp.Transactions) > pageSize {
			t.Fatalf("page returned %d transactions, want <= %d", len(resp.Transactions), pageSize)
		}
		for _, tx := range resp.Transactions {
			got = append(got, tx.Ref)
		}
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	want := []string{"tx-4", "tx-3", "tx-2", "tx-1", "tx-0"}
	if len(got) != len(want) {
		t.Fatalf("paginated refs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paginated refs = %v, want %v", got, want)
		}
	}
}

// TestTransactionRepositoryImpl_ListInvalidCursor proves a malformed cursor is
// reported as a caller error rather than silently returning an empty page.
func TestTransactionRepositoryImpl_ListInvalidCursor(t *testing.T) {
	db := newTestDB(t)
	createTestAccount(t, db, "page-acct")
	repo := NewTransactionRepositoryImpl(db)

	_, err := repo.List(context.Background(), pkgWallet.ListTransactionsRequest{UserID: "user-page-acct", Cursor: "!!!not-base64!!!"})
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("List(bad cursor) error = %v, want errs.ErrInvalidArgument", err)
	}
}

func TestTransactionCursorRoundTrip(t *testing.T) {
	recordedAt := "2026-06-01T10:00:01Z"
	ref := "tx-001"

	gotTS, gotRef, err := decodeTransactionCursor(encodeTransactionCursor(recordedAt, ref))
	if err != nil {
		t.Fatalf("decode(encode()) error = %v", err)
	}
	if gotTS != recordedAt || gotRef != ref {
		t.Fatalf("round trip = (%q, %q), want (%q, %q)", gotTS, gotRef, recordedAt, ref)
	}
}

func TestDecodeTransactionCursorRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"not base64":        "%%%",
		"missing separator": base64.URLEncoding.EncodeToString([]byte("2026-06-01T10:00:01Z")),
		"empty ref":         base64.URLEncoding.EncodeToString([]byte("2026-06-01T10:00:01Z\x00")),
		"bad timestamp":     base64.URLEncoding.EncodeToString([]byte("not-a-time\x00tx-001")),
	}
	for name, cursor := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := decodeTransactionCursor(cursor); !errors.Is(err, errs.ErrInvalidArgument) {
				t.Fatalf("decode(%q) error = %v, want errs.ErrInvalidArgument", cursor, err)
			}
		})
	}
}

func TestClampPageSize(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, defaultPageSize},
		{-5, defaultPageSize},
		{10, 10},
		{maxPageSize, maxPageSize},
		{maxPageSize + 1, maxPageSize},
	}
	for _, c := range cases {
		if got := clampPageSize(c.in); got != c.want {
			t.Fatalf("clampPageSize(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestTransactionRepositoryImpl_GetByIDNotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewTransactionRepositoryImpl(db)

	_, err := repo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "missing", UserID: "user-1"})
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

	if _, err := accountRepo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123", UserID: "user-1"}); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("account survived rollback: error = %v, want errs.ErrNotFound", err)
	}
	if _, err := transactionRepo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-001", UserID: "user-member-123"}); !errors.Is(err, errs.ErrNotFound) {
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

	if _, err := accountRepo.GetByID(ctx, pkgAccounts.GetAccountByIDRequest{AccountID: "member-123", UserID: "user-1"}); err != nil {
		t.Fatalf("account not committed: %v", err)
	}
	if _, err := transactionRepo.GetByID(ctx, pkgWallet.GetTransactionByIDRequest{Ref: "tx-001", UserID: "user-member-123"}); err != nil {
		t.Fatalf("transaction not committed: %v", err)
	}
}

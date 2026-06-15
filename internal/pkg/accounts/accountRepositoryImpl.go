package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
)

// AccountRepositoryImpl is the Postgres implementation of
// accounts.AccountRepository. Every method resolves its executor from the
// context, so it runs inside an ambient transaction when one is present and
// against the pool otherwise.
type AccountRepositoryImpl struct {
	db *sql.DB
}

func NewAccountRepositoryImpl(db *sql.DB) *AccountRepositoryImpl {
	return &AccountRepositoryImpl{db: db}
}

func (r *AccountRepositoryImpl) Create(ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for Create: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	account := request.Account
	if account.ID == "" {
		account.ID = uuid.NewString()
	}
	_, err := exec.ExecContext(ctx,
		`INSERT INTO accounts (id, owner_id, name, balance, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		account.ID,
		account.OwnerID,
		account.Name,
		account.Balance,
		time.FormatTime(account.CreatedAt),
	)
	if err != nil {
		if postgres.IsUniqueConstraintViolation(err) {
			return nil, fmt.Errorf("account %s: %w", account.ID, errs.ErrAlreadyExists)
		}
		return nil, fmt.Errorf("could not insert account: %w", err)
	}

	return &pkgAccounts.CreateAccountResponse{Account: account}, nil
}

func (r *AccountRepositoryImpl) List(ctx context.Context, request pkgAccounts.ListAccountsRequest) (*pkgAccounts.ListAccountsResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for List: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping mirrors GetByID: holding account:read:all lists every
	// owner's accounts; otherwise the WHERE clause restricts the listing to
	// request.UserID.
	query := `SELECT id, owner_id, name, balance, created_at
		 FROM accounts`
	var args []any
	if !authorization.IsGranted(ctx, authorization.PermAccountReadAll) {
		query += ` WHERE owner_id = $1`
		args = append(args, request.UserID)
	}
	query += ` ORDER BY created_at, id`

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []pkgAccounts.Account
	for rows.Next() {
		account, err := scanAccount(rows.Scan)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate accounts: %w", err)
	}

	return &pkgAccounts.ListAccountsResponse{Accounts: accounts}, nil
}

func (r *AccountRepositoryImpl) GetByID(ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetByID: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping is driven by the caller's permissions: holding
	// account:read:all reads across owners; otherwise (own-scope only, or no
	// account:read:all grant — e.g. an internal caller) the WHERE clause restricts
	// the row to request.UserID, so a non-owner sees the same ErrNotFound as for a
	// missing account.
	query := `SELECT id, owner_id, name, balance, created_at
		 FROM accounts
		 WHERE id = $1`
	args := []any{request.AccountID}
	if !authorization.IsGranted(ctx, authorization.PermAccountReadAll) {
		query += ` AND owner_id = $2`
		args = append(args, request.UserID)
	}

	row := exec.QueryRowContext(ctx, query, args...)

	account, err := scanAccount(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account %s: %w", request.AccountID, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgAccounts.GetAccountByIDResponse{Account: *account}, nil
}

func (r *AccountRepositoryImpl) GetAccountBalance(ctx context.Context, request pkgAccounts.GetAccountBalanceRequest) (*pkgAccounts.GetAccountBalanceResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetAccountBalance: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping mirrors GetByID: holding account:read:all reads across
	// owners; otherwise the read is restricted to request.UserID and a non-owner
	// gets ErrNotFound.
	query := `SELECT balance FROM accounts WHERE id = $1`
	args := []any{request.AccountID}
	if !authorization.IsGranted(ctx, authorization.PermAccountReadAll) {
		query += ` AND owner_id = $2`
		args = append(args, request.UserID)
	}

	var balance int64
	err := exec.QueryRowContext(ctx, query, args...).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("account %s: %w", request.AccountID, errs.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("could not read account balance: %w", err)
	}

	return &pkgAccounts.GetAccountBalanceResponse{Balance: balance}, nil
}

func (r *AccountRepositoryImpl) UpdateAccountBalance(ctx context.Context, request pkgAccounts.UpdateAccountBalanceRequest) (*pkgAccounts.UpdateAccountBalanceResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for UpdateAccountBalance: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Single atomic, overdraft-guarded statement: the WHERE clause makes the
	// read-check-write indivisible. Unless the caller holds account:write:all the
	// same clause also pins the row to request.UserID (owner_id), so the
	// ownership check is enforced in the very statement that mutates — a
	// non-owner's update matches no row. On Postgres the row is locked for the
	// life of the surrounding transaction, so two concurrent debits to the same
	// account serialize at the row rather than the whole database. The CHECK
	// constraint on the column is the backstop.
	scoped := !authorization.IsGranted(ctx, authorization.PermAccountWriteAll)
	query := `UPDATE accounts
		 SET balance = balance + $1
		 WHERE id = $2 AND balance + $1 >= 0`
	args := []any{request.Delta, request.AccountID}
	if scoped {
		query += ` AND owner_id = $3`
		args = append(args, request.UserID)
	}

	result, err := exec.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not update account balance: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("could not read affected rows: %w", err)
	}

	if affected == 0 {
		// Zero rows means the account is missing, not owned by this user, or the
		// guard rejected the delta. Re-read with the same ownership scope: no row
		// means missing-or-not-owned (reported as ErrNotFound, no existence leak,
		// mirroring GetByID); a row means the balance guard rejected the delta.
		checkQuery := `SELECT balance FROM accounts WHERE id = $1`
		checkArgs := []any{request.AccountID}
		if scoped {
			checkQuery += ` AND owner_id = $2`
			checkArgs = append(checkArgs, request.UserID)
		}
		var balance int64
		err := exec.QueryRowContext(ctx, checkQuery, checkArgs...).Scan(&balance)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account %s: %w", request.AccountID, errs.ErrNotFound)
		}
		if err != nil {
			return nil, fmt.Errorf("could not read account balance: %w", err)
		}
		return nil, fmt.Errorf("account %s: %w", request.AccountID, errs.ErrInsufficientBalance)
	}

	var balance int64
	if err := exec.QueryRowContext(ctx,
		`SELECT balance FROM accounts WHERE id = $1`,
		request.AccountID,
	).Scan(&balance); err != nil {
		return nil, fmt.Errorf("could not read updated account balance: %w", err)
	}

	return &pkgAccounts.UpdateAccountBalanceResponse{Balance: balance}, nil
}
func (r *AccountRepositoryImpl) UpdateAccountName(ctx context.Context, request pkgAccounts.UpdateAccountNameRequest) (*pkgAccounts.UpdateAccountNameResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for UpdateAccountName: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Single statement renames and returns the updated row. Ownership scoping
	// mirrors UpdateAccountBalance: unless the caller holds account:write:all the
	// WHERE clause pins the row to request.UserID (owner_id), so a non-owner's
	// update matches no row and reads as ErrNotFound — no existence leak.
	query := `UPDATE accounts
		 SET name = $1
		 WHERE id = $2`
	args := []any{request.Name, request.AccountID}
	if !authorization.IsGranted(ctx, authorization.PermAccountWriteAll) {
		query += ` AND owner_id = $3`
		args = append(args, request.UserID)
	}
	query += ` RETURNING id, owner_id, name, balance, created_at`

	account, err := scanAccount(exec.QueryRowContext(ctx, query, args...).Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account %s: %w", request.AccountID, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgAccounts.UpdateAccountNameResponse{Account: *account}, nil
}

func scanAccount(scan func(dest ...any) error) (*pkgAccounts.Account, error) {
	var account pkgAccounts.Account
	var createdAt string

	if err := scan(
		&account.ID,
		&account.OwnerID,
		&account.Name,
		&account.Balance,
		&createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("could not scan account: %w", err)
	}

	parsedCreatedAt, err := time.ParseTime(createdAt)
	if err != nil {
		return nil, err
	}
	account.CreatedAt = parsedCreatedAt

	return &account, nil
}

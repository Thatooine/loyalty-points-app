package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/sqlite"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
)

// AccountRepositoryImpl is the SQLite implementation of
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
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	account := request.Account
	_, err := exec.ExecContext(ctx,
		`INSERT INTO accounts (account_id, user_id, name, balance, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		account.AccountID,
		account.UserID,
		account.Name,
		account.Balance,
		time.FormatTime(account.CreatedAt),
	)
	if err != nil {
		if sqlite.IsUniqueConstraintViolation(err) {
			return nil, fmt.Errorf("account %s: %w", account.AccountID, errs.ErrAlreadyExists)
		}
		return nil, fmt.Errorf("could not insert account: %w", err)
	}

	return &pkgAccounts.CreateAccountResponse{Account: account}, nil
}

func (r *AccountRepositoryImpl) List(ctx context.Context, request pkgAccounts.ListAccountsRequest) (*pkgAccounts.ListAccountsResponse, error) {
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	rows, err := exec.QueryContext(ctx,
		`SELECT account_id, user_id, name, balance, created_at
		 FROM accounts
		 ORDER BY created_at, account_id`,
	)
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
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	row := exec.QueryRowContext(ctx,
		`SELECT account_id, user_id, name, balance, created_at
		 FROM accounts
		 WHERE account_id = ?`,
		request.AccountID,
	)

	account, err := scanAccount(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account %s: %w", request.AccountID, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgAccounts.GetAccountByIDResponse{Account: *account}, nil
}

func scanAccount(scan func(dest ...any) error) (*pkgAccounts.Account, error) {
	var account pkgAccounts.Account
	var createdAt string

	if err := scan(
		&account.AccountID,
		&account.UserID,
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

package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

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
		`INSERT INTO accounts (id, user_id, name, balance, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		account.ID,
		account.UserID,
		account.Name,
		account.Balance,
		time.FormatTime(account.CreatedAt),
	)
	if err != nil {
		if sqlite.IsUniqueConstraintViolation(err) {
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

	rows, err := exec.QueryContext(ctx,
		`SELECT id, user_id, name, balance, created_at
		 FROM accounts
		 ORDER BY created_at, id`,
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
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetByID: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	row := exec.QueryRowContext(ctx,
		`SELECT id, user_id, name, balance, created_at
		 FROM accounts
		 WHERE id = ?`,
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

func (r *AccountRepositoryImpl) UpdateAccountBalance(ctx context.Context, request pkgAccounts.UpdateAccountBalanceRequest) (*pkgAccounts.UpdateAccountBalanceResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for UpdateAccountBalance: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Single atomic, overdraft-guarded statement: the WHERE clause makes the
	// read-check-write indivisible, so two concurrent debits can never both
	// pass a balance check and drive the total negative. The CHECK constraint
	// on the column is the backstop.
	result, err := exec.ExecContext(ctx,
		`UPDATE accounts
		 SET balance = balance + ?
		 WHERE id = ? AND balance + ? >= 0`,
		request.Delta,
		request.AccountID,
		request.Delta,
	)
	if err != nil {
		return nil, fmt.Errorf("could not update account balance: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("could not read affected rows: %w", err)
	}

	if affected == 0 {
		// Zero rows means either the account is missing or the guard rejected
		// the delta. Read the current balance to tell the two apart.
		var balance int64
		err := exec.QueryRowContext(ctx,
			`SELECT balance FROM accounts WHERE id = ?`,
			request.AccountID,
		).Scan(&balance)
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
		`SELECT balance FROM accounts WHERE id = ?`,
		request.AccountID,
	).Scan(&balance); err != nil {
		return nil, fmt.Errorf("could not read updated account balance: %w", err)
	}

	return &pkgAccounts.UpdateAccountBalanceResponse{Balance: balance}, nil
}

func scanAccount(scan func(dest ...any) error) (*pkgAccounts.Account, error) {
	var account pkgAccounts.Account
	var createdAt string

	if err := scan(
		&account.ID,
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

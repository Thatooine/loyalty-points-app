package accounts

import "context"

// AccountRepository is the persistence port for Account entities. Methods
// participate in an ambient transaction when one is present in the context
// (see sqlite.TransactionManager), and run against the pool otherwise.
type AccountRepository interface {
	// Create persists a new account. An existing account with the same
	// AccountID results in errs.ErrAlreadyExists.
	Create(ctx context.Context, request CreateAccountRequest) (*CreateAccountResponse, error)

	// List returns all accounts, oldest first.
	List(ctx context.Context, request ListAccountsRequest) (*ListAccountsResponse, error)

	// GetByID returns the account with the given AccountID, or
	// errs.ErrNotFound. When UserID is set the lookup is ownership-scoped: an
	// account that exists but is owned by another user is reported as
	// errs.ErrNotFound, indistinguishable from a missing account so callers
	// cannot probe for accounts they do not own.
	GetByID(ctx context.Context, request GetAccountByIDRequest) (*GetAccountByIDResponse, error)

	// GetAccountBalance returns just the balance of the given account, or
	// errs.ErrNotFound. Like GetByID it is ownership-scoped when UserID is set.
	GetAccountBalance(ctx context.Context, request GetAccountBalanceRequest) (*GetAccountBalanceResponse, error)

	// UpdateAccountBalance applies a signed delta to an account balance in a
	// single atomic, overdraft-guarded statement (the only intent-revealing
	// mutator beyond CRUD). It returns the new balance, or errs.ErrNotFound if
	// the account does not exist, or errs.ErrInsufficientBalance if the delta
	// would drive the balance below zero (balance left unchanged).
	UpdateAccountBalance(ctx context.Context, request UpdateAccountBalanceRequest) (*UpdateAccountBalanceResponse, error)
}

// CreateAccountRequest is the request for Create.
type CreateAccountRequest struct {
	Account Account
}

// CreateAccountResponse is the response for Create.
type CreateAccountResponse struct {
	Account Account
}

// ListAccountsRequest is the request for List.
type ListAccountsRequest struct {
}

// ListAccountsResponse is the response for List.
type ListAccountsResponse struct {
	Accounts []Account
}

// GetAccountByIDRequest is the request for GetByID.
type GetAccountByIDRequest struct {
	AccountID string

	// UserID, when non-empty, scopes the lookup to the owning user so the
	// account is only returned to its owner. Leave empty for internal/admin
	// lookups that must read any account.
	UserID string
}

// GetAccountByIDResponse is the response for GetByID.
type GetAccountByIDResponse struct {
	Account Account
}

// GetAccountBalanceRequest is the request for GetAccountBalance.
type GetAccountBalanceRequest struct {
	AccountID string

	// UserID, when non-empty, scopes the lookup to the owning user (see
	// GetAccountByIDRequest.UserID).
	UserID string
}

// GetAccountBalanceResponse is the response for GetAccountBalance.
type GetAccountBalanceResponse struct {
	Balance int64
}

// UpdateAccountBalanceRequest is the request for UpdateAccountBalance.
type UpdateAccountBalanceRequest struct {
	AccountID string
	// Delta is the signed amount to apply: positive to credit, negative to
	// debit.
	Delta int64
}

// UpdateAccountBalanceResponse is the response for UpdateAccountBalance.
type UpdateAccountBalanceResponse struct {
	// Balance is the account balance after the delta was applied.
	Balance int64
}

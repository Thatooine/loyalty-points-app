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
	// errs.ErrNotFound.
	GetByID(ctx context.Context, request GetAccountByIDRequest) (*GetAccountByIDResponse, error)

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
}

// GetAccountByIDResponse is the response for GetByID.
type GetAccountByIDResponse struct {
	Account Account
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

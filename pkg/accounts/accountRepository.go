package accounts

import "context"

type AccountRepository interface {
	Create(ctx context.Context, request CreateAccountRequest) (*CreateAccountResponse, error)

	List(ctx context.Context, request ListAccountsRequest) (*ListAccountsResponse, error)

	GetByID(ctx context.Context, request GetAccountByIDRequest) (*GetAccountByIDResponse, error)

	GetAccountBalance(ctx context.Context, request GetAccountBalanceRequest) (*GetAccountBalanceResponse, error)

	UpdateAccountBalance(ctx context.Context, request UpdateAccountBalanceRequest) (*UpdateAccountBalanceResponse, error)

	UpdateAccountName(ctx context.Context, request UpdateAccountNameRequest) (*UpdateAccountNameResponse, error)
}

type CreateAccountRequest struct {
	Account Account
}

type CreateAccountResponse struct {
	Account Account
}

type ListAccountsRequest struct {
	// UserID, when non-empty, scopes the listing to the owning user so only
	// that user's accounts are returned (see GetAccountByIDRequest.UserID).
	// Leave empty for internal/admin listings that must see every account.
	UserID string
}

type ListAccountsResponse struct {
	Accounts []Account
}

type GetAccountByIDRequest struct {
	AccountID string

	// UserID, when non-empty, scopes the lookup to the owning user so the
	// account is only returned to its owner. Leave empty for internal/admin
	// lookups that must read any account.
	UserID string
}

type GetAccountByIDResponse struct {
	Account Account
}

type GetAccountBalanceRequest struct {
	AccountID string

	// UserID, when non-empty, scopes the lookup to the owning user (see
	// GetAccountByIDRequest.UserID).
	UserID string
}

type GetAccountBalanceResponse struct {
	Balance int64
}

type UpdateAccountBalanceRequest struct {
	AccountID string
	// Delta is the signed amount to apply: positive to credit, negative to
	// debit.
	Delta int64

	// UserID, when non-empty, scopes the update to the owning user so the
	// balance is only mutated on an account that user owns (see
	// GetAccountByIDRequest.UserID). Leave empty for internal/admin updates that
	// must act on any account.
	UserID string
}

type UpdateAccountBalanceResponse struct {
	// Balance is the account balance after the delta was applied.
	Balance int64
}

type UpdateAccountNameRequest struct {
	AccountID string

	// Name is the new display name for the account.
	Name string

	// UserID, when non-empty, scopes the update to the owning user so the name
	// is only mutated on an account that user owns (see
	// GetAccountByIDRequest.UserID). Leave empty for internal/admin updates that
	// must act on any account.
	UserID string
}

type UpdateAccountNameResponse struct {
	// Account is the account after the rename.
	Account Account
}

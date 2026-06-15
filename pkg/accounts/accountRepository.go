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
	// UserID, when non-empty, scopes the listing to the owning user so only
	// that user's accounts are returned (see GetAccountByIDRequest.UserID).
	// Leave empty for internal/admin listings that must see every account.
	UserID string
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

	// UserID, when non-empty, scopes the update to the owning user so the
	// balance is only mutated on an account that user owns (see
	// GetAccountByIDRequest.UserID). Leave empty for internal/admin updates that
	// must act on any account.
	UserID string
}

// UpdateAccountBalanceResponse is the response for UpdateAccountBalance.
type UpdateAccountBalanceResponse struct {
	// Balance is the account balance after the delta was applied.
	Balance int64
}

// UpdateAccountNameRequest is the request for UpdateAccountName.
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

// UpdateAccountNameResponse is the response for UpdateAccountName.
type UpdateAccountNameResponse struct {
	// Account is the account after the rename.
	Account Account
}

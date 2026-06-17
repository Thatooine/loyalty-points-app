package accounts

import "context"

// AccountService is the domain entry point for reading and mutating accounts. It
// fronts AccountRepository so the transport layer depends on a service port
// rather than a persistence component; for now it validates the request and
// delegates 1:1 to the repository, holding no logic of its own.
//
// Every request carries a UserID scope, resolved by the adaptor from the
// verified login claim: without the matching ":all" permission the repository
// scopes the operation to that user, so a caller only ever reads or mutates
// accounts they own.
type AccountService interface {
	GetAccountByID(ctx context.Context, request ReadAccountRequest) (*ReadAccountResponse, error)

	GetAccountBalance(ctx context.Context, request ReadAccountBalanceRequest) (*ReadAccountBalanceResponse, error)

	UpdateAccountName(ctx context.Context, request RenameAccountRequest) (*RenameAccountResponse, error)

	UpdateAccountBalance(ctx context.Context, request AdjustAccountBalanceRequest) (*AdjustAccountBalanceResponse, error)
}

type ReadAccountRequest struct {
	UserID string

	AccountID string
}

type ReadAccountResponse struct {
	Account Account
}

type ReadAccountBalanceRequest struct {
	UserID string

	AccountID string
}

type ReadAccountBalanceResponse struct {
	Balance int64
}

type RenameAccountRequest struct {
	UserID string

	AccountID string

	Name string
}

type RenameAccountResponse struct {
	// Account is the account after the rename.
	Account Account
}

type AdjustAccountBalanceRequest struct {
	UserID string

	AccountID string

	Delta int64
}

type AdjustAccountBalanceResponse struct {
	// Balance is the account balance after the delta was applied.
	Balance int64
}

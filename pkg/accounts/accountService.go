package accounts

import "context"

type AccountService interface {
	FetchMyAccounts(ctx context.Context, request FetchMyAccountsRequest) (*FetchMyAccountsResponse, error)

	GetAccountByID(ctx context.Context, request ReadAccountRequest) (*ReadAccountResponse, error)

	GetAccountBalance(ctx context.Context, request ReadAccountBalanceRequest) (*ReadAccountBalanceResponse, error)

	UpdateAccountName(ctx context.Context, request RenameAccountRequest) (*RenameAccountResponse, error)

	UpdateAccountBalance(ctx context.Context, request AdjustAccountBalanceRequest) (*AdjustAccountBalanceResponse, error)
}

type FetchMyAccountsRequest struct {
	UserID string
}

type FetchMyAccountsResponse struct {
	Accounts []Account
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
	Account Account
}

type AdjustAccountBalanceRequest struct {
	UserID string

	AccountID string

	Delta int64
}

type AdjustAccountBalanceResponse struct {
	Balance int64
}

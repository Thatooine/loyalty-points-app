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
	UserID string
}

type ListAccountsResponse struct {
	Accounts []Account
}

type GetAccountByIDRequest struct {
	AccountID string

	UserID string
}

type GetAccountByIDResponse struct {
	Account Account
}

type GetAccountBalanceRequest struct {
	AccountID string

	UserID string
}

type GetAccountBalanceResponse struct {
	Balance int64
}

type UpdateAccountBalanceRequest struct {
	AccountID string

	Delta int64

	UserID string
}

type UpdateAccountBalanceResponse struct {
	Balance int64
}

type UpdateAccountNameRequest struct {
	AccountID string

	Name string

	UserID string
}

type UpdateAccountNameResponse struct {
	Account Account
}

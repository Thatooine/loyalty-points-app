package accounts

import "context"

type AccountOpener interface {
	OpenAccount(ctx context.Context, request OpenAccountRequest) (*OpenAccountResponse, error)
}

type OpenAccountRequest struct {
	UserID string

	// Name is optional; the service substitutes a default when empty.
	Name string
}

type OpenAccountResponse struct {
	Account Account
}

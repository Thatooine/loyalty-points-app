package accounts

import "context"

// AccountOpener opens a new loyalty-points wallet for the calling user. It is a
// thin domain service over AccountRepository.Create: it owns the policy that the
// repository deliberately does not — defaulting a blank name and forcing the new
// account's owner to the caller — so no caller can open an account for anyone
// else.
type AccountOpener interface {
	OpenAccount(ctx context.Context, request OpenAccountRequest) (*OpenAccountResponse, error)
}

// OpenAccountRequest is the request for OpenAccount.
type OpenAccountRequest struct {
	// UserID is the owner of the new account. It is always the calling
	// principal — the adaptor fills it from the verified login claim, never
	// from the wire — so a caller can only open accounts for themselves.
	UserID string

	// Name is the display name for the new account. When empty the service
	// substitutes a default, so it is optional on the wire.
	Name string
}

// OpenAccountResponse is the response for OpenAccount.
type OpenAccountResponse struct {
	// Account is the freshly opened account, including its assigned ID and a
	// starting balance of zero.
	Account Account
}

package accounts

import "github.com/Thatooine/loyalty-points-app/pkg/errs"

// Validate checks the OpenAccount request. Only UserID is required: it is the
// owner the account is opened for and is pinned to the caller upstream. Name is
// optional because the service defaults it when blank.
func (r *OpenAccountRequest) Validate() error {
	var reasons []string

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

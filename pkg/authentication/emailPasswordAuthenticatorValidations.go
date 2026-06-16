package authentication

import "github.com/Thatooine/loyalty-points-app/pkg/errs"

func (r *EmailPasswordAuthenticatorRequest) Validate() error {
	var reasons []string

	if r.Email == "" {
		reasons = append(reasons, "Email is required")
	}

	if r.Password == "" {
		reasons = append(reasons, "Password is required")
	}

	return errs.NewValidationError(reasons)
}

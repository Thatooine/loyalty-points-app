package users

import (
	"fmt"
	"strings"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// minPasswordLength is the shortest password we accept at registration.
const minPasswordLength = 8

func (r *RegisterRequest) Validate() error {
	var reasons []string

	if r.Email == "" {
		reasons = append(reasons, "Email is required")
	} else if !strings.Contains(r.Email, "@") {
		reasons = append(reasons, "Email must be a valid email address")
	}

	if r.Password == "" {
		reasons = append(reasons, "Password is required")
	} else if len(r.Password) < minPasswordLength {
		reasons = append(reasons, fmt.Sprintf("Password must be at least %d characters", minPasswordLength))
	}

	if r.Name == "" {
		reasons = append(reasons, "Name is required")
	}

	return errs.NewValidationError(reasons)
}

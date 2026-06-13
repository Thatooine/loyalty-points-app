package authentication

import (
	"fmt"
	"strings"
)

func (r *EmailPasswordAuthenticatorRequest) Validate() error {
	var reasons []string

	if r.Email == "" {
		reasons = append(reasons, "Email is required")
	}

	if r.Password == "" {
		reasons = append(reasons, "Password is required")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

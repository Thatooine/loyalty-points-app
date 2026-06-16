package accounts

import (
	"fmt"
	"strings"
)

// Validate checks the OpenAccount request. Only UserID is required: it is the
// owner the account is opened for and is pinned to the caller upstream. Name is
// optional because the service defaults it when blank.
func (r *OpenAccountRequest) Validate() error {
	var reasons []string

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

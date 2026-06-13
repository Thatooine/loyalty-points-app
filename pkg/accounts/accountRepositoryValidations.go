package accounts

import (
	"fmt"
	"strings"
)

func (r *CreateAccountRequest) Validate() error {
	var reasons []string

	if r.Account.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	if r.Account.Name == "" {
		reasons = append(reasons, "Name is required")
	}

	if r.Account.Balance < 0 {
		reasons = append(reasons, "Balance must be >= 0")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

func (r *GetAccountByIDRequest) Validate() error {
	var reasons []string

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

func (r *UpdateAccountBalanceRequest) Validate() error {
	var reasons []string

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.Delta == 0 {
		reasons = append(reasons, "Delta must be non-zero")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

// Validate has no fields to check; defined for a uniform call site.
func (r *ListAccountsRequest) Validate() error {
	return nil
}

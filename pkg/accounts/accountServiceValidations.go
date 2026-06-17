package accounts

import "github.com/Thatooine/loyalty-points-app/pkg/errs"

func (r *ReadAccountRequest) Validate() error {
	var reasons []string

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

func (r *ReadAccountBalanceRequest) Validate() error {
	var reasons []string

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

func (r *RenameAccountRequest) Validate() error {
	var reasons []string

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.Name == "" {
		reasons = append(reasons, "Name is required")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

func (r *AdjustAccountBalanceRequest) Validate() error {
	var reasons []string

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.Delta == 0 {
		reasons = append(reasons, "Delta must be non-zero")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

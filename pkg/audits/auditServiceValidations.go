package audits

import "github.com/Thatooine/loyalty-points-app/pkg/errs"

func (r *ListAuditByRefRequest) Validate() error {
	var reasons []string

	if r.TransactionRef == "" {
		reasons = append(reasons, "TransactionRef is required")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

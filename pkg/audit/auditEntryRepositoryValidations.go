package audit

import "github.com/Thatooine/loyalty-points-app/pkg/errs"

func (r *CreateAuditEntryRequest) Validate() error {
	var reasons []string

	if r.AuditEntry.Reason == "" {
		reasons = append(reasons, "Reason is required")
	}

	if r.AuditEntry.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	switch r.AuditEntry.Outcome {
	case OutcomeAccepted, OutcomeRejected, OutcomeDuplicate:
	default:
		reasons = append(reasons, "Outcome must be 'accepted', 'rejected' or 'duplicate'")
	}

	return errs.NewValidationError(reasons)
}

func (r *GetAuditEntryByIDRequest) Validate() error {
	var reasons []string

	if r.ID <= 0 {
		reasons = append(reasons, "ID must be greater than 0")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

func (r *ListAuditEntriesRequest) Validate() error {
	var reasons []string

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

func (r *ListAuditEntriesByTransactionRefRequest) Validate() error {
	var reasons []string

	if r.TransactionRef == "" {
		reasons = append(reasons, "TransactionRef is required")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

func (r *ListAuditEntriesByAccountIDRequest) Validate() error {
	var reasons []string

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	return errs.NewValidationError(reasons)
}

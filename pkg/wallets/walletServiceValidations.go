package wallets

import "github.com/Thatooine/loyalty-points-app/pkg/errs"

func (r *ProcessTransactionRequest) Validate() error {
	var reasons []string

	if r.Ref == "" {
		reasons = append(reasons, "Ref is required")
	}

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	// Points is supplied positive for earn/spend; the sign is derived from Kind.
	switch r.Kind {
	case KindEarn, KindSpend:
		if r.Points <= 0 {
			reasons = append(reasons, "Points must be greater than 0 for earn and spend")
		}
	default:
		reasons = append(reasons, "Kind must be 'earn' or 'spend'")
	}

	return errs.NewValidationError(reasons)
}

package wallets

import (
	"fmt"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// maxBatchSize bounds how many transactions a single batch may carry, so one
// request cannot enqueue unbounded work. The transport-level body-size limit is
// the first line of defence; this is the explicit semantic cap.
const maxBatchSize = 1000

func (r *ProcessTransactionBatchRequest) Validate() error {
	var reasons []string

	switch {
	case len(r.Transactions) == 0:
		reasons = append(reasons, "batch must contain at least one transaction")
	case len(r.Transactions) > maxBatchSize:
		reasons = append(reasons, fmt.Sprintf("batch exceeds maximum of %d transactions", maxBatchSize))
	}

	return errs.NewValidationError(reasons)
}

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

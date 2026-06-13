package wallet

import (
	"fmt"
	"strings"
)

func (r *CreateTransactionRequest) Validate() error {
	var reasons []string

	if r.Transaction.Ref == "" {
		reasons = append(reasons, "Ref is required")
	}

	if r.Transaction.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.Transaction.CreatedBy == "" {
		reasons = append(reasons, "CreatedBy is required")
	}

	switch r.Transaction.Kind {
	case KindEarn, KindSpend, KindAdjust:
	default:
		reasons = append(reasons, "Kind must be 'earn', 'spend' or 'adjust'")
	}

	// Points is the signed delta as applied, so it may be negative (spend /
	// adjust) but never zero.
	if r.Transaction.Points == 0 {
		reasons = append(reasons, "Points must be non-zero")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

func (r *GetTransactionByIDRequest) Validate() error {
	var reasons []string

	if r.Ref == "" {
		reasons = append(reasons, "Ref is required")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

// Validate has no fields to check; defined for a uniform call site.
func (r *ListTransactionsRequest) Validate() error {
	return nil
}

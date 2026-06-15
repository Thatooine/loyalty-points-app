package wallet

import (
	"fmt"
	"strings"
)

func (r *ProcessTransactionRequest) Validate() error {
	var reasons []string

	if r.Ref == "" {
		reasons = append(reasons, "Ref is required")
	}

	if r.AccountID == "" {
		reasons = append(reasons, "AccountID is required")
	}

	if r.Actor == "" {
		reasons = append(reasons, "Actor is required")
	}

	// Points is supplied positive for earn/spend (the sign is derived from
	// Kind); for adjust it is the signed delta and may be negative, never zero.
	switch r.Kind {
	case KindEarn, KindSpend:
		if r.Points <= 0 {
			reasons = append(reasons, "Points must be greater than 0 for earn and spend")
		}
	case KindAdjust:
		if r.Points == 0 {
			reasons = append(reasons, "Points must be non-zero for adjust")
		}
	default:
		reasons = append(reasons, "Kind must be 'earn', 'spend' or 'adjust'")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

package wallets

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
	case KindEarn, KindSpend:
	default:
		reasons = append(reasons, "Kind must be 'earn' or 'spend'")
	}

	// Points is the signed delta as applied, so it may be negative (spend) but
	// never zero.
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

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

func (r *ListTransactionsRequest) Validate() error {
	var reasons []string

	if r.UserID == "" {
		reasons = append(reasons, "UserID is required")
	}

	// Zero is allowed and means "server default"; only a negative page size is a
	// caller error. The repository clamps positive values to its maximum.
	if r.PageSize < 0 {
		reasons = append(reasons, "PageSize must be >= 0")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

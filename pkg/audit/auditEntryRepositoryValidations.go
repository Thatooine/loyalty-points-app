package audit

import (
	"fmt"
	"strings"
)

func (r *CreateAuditEntryRequest) Validate() error {
	var reasons []string

	if r.AuditEntry.Source == "" {
		reasons = append(reasons, "Source is required")
	}

	if r.AuditEntry.Reason == "" {
		reasons = append(reasons, "Reason is required")
	}

	if r.AuditEntry.Actor == "" {
		reasons = append(reasons, "Actor is required")
	}

	switch r.AuditEntry.Outcome {
	case OutcomeAccepted, OutcomeRejected, OutcomeDuplicate:
	default:
		reasons = append(reasons, "Outcome must be 'accepted', 'rejected' or 'duplicate'")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

func (r *GetAuditEntryByIDRequest) Validate() error {
	var reasons []string

	if r.ID <= 0 {
		reasons = append(reasons, "ID must be greater than 0")
	}

	if len(reasons) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(reasons, "; "))
	}

	return nil
}

// Validate has no fields to check; defined for a uniform call site.
func (r *ListAuditEntriesRequest) Validate() error {
	return nil
}

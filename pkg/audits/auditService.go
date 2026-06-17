package audits

import "context"

// AuditService is the domain entry point for reading the audit trail. It fronts
// AuditEntryRepository so the transport layer depends on a service port rather
// than a persistence component; for now it validates the request and delegates
// 1:1 to the repository, holding no logic of its own.
//
// The request carries a UserID scope, resolved by the adaptor from the verified
// login claim: without the audit:read:all permission the repository scopes the
// listing to that user, so a member only ever sees attempts recorded against
// their own accounts.
type AuditService interface {
	ListByTransactionRef(ctx context.Context, request ListAuditByRefRequest) (*ListAuditByRefResponse, error)
}

type ListAuditByRefRequest struct {
	TransactionRef string

	// UserID scopes the listing to the owning user; the adaptor fills it from the
	// verified login claim, never from the wire.
	UserID string
}

type ListAuditByRefResponse struct {
	AuditEntries []AuditEntry
}

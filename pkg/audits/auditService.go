package audits

import "context"

type AuditService interface {
	FetchTransactionAuditTrail(ctx context.Context, request ListAuditByRefRequest) (*ListAuditByRefResponse, error)
}

type ListAuditByRefRequest struct {
	UserID string

	TransactionRef string
}

type ListAuditByRefResponse struct {
	AuditEntries []AuditEntry
}

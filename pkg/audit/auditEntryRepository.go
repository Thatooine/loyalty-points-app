package audit

import "context"

// AuditEntryRepository is the persistence port for the audit trail. Methods
// participate in an ambient transaction when one is present in the context
// (see sql.TxManager), and run against the pool otherwise —
// rejected attempts are audited outside the rolled-back transaction so the
// trail survives the rejection.
type AuditEntryRepository interface {
	// Create persists an audit entry and returns it with its assigned ID.
	Create(ctx context.Context, request CreateAuditEntryRequest) (*CreateAuditEntryResponse, error)

	// List returns all audit entries, oldest first.
	List(ctx context.Context, request ListAuditEntriesRequest) (*ListAuditEntriesResponse, error)

	// GetByID returns the audit entry with the given ID, or errs.ErrNotFound.
	GetByID(ctx context.Context, request GetAuditEntryByIDRequest) (*GetAuditEntryByIDResponse, error)
}

// CreateAuditEntryRequest is the request for Create.
type CreateAuditEntryRequest struct {
	AuditEntry AuditEntry
}

// CreateAuditEntryResponse is the response for Create.
type CreateAuditEntryResponse struct {
	AuditEntry AuditEntry
}

// ListAuditEntriesRequest is the request for List.
type ListAuditEntriesRequest struct {
}

// ListAuditEntriesResponse is the response for List.
type ListAuditEntriesResponse struct {
	AuditEntries []AuditEntry
}

// GetAuditEntryByIDRequest is the request for GetByID.
type GetAuditEntryByIDRequest struct {
	ID int64
}

// GetAuditEntryByIDResponse is the response for GetByID.
type GetAuditEntryByIDResponse struct {
	AuditEntry AuditEntry
}

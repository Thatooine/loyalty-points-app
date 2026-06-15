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

	// ListByTransactionRef returns every audit entry recorded for the given
	// transaction ref, oldest first. Unlike the ledger a ref can appear many
	// times (one per processing attempt), so this returns all of them. An empty
	// slice is returned when none exist; it is not an error.
	ListByTransactionRef(ctx context.Context, request ListAuditEntriesByTransactionRefRequest) (*ListAuditEntriesByTransactionRefResponse, error)

	// ListByAccountID returns every audit entry recorded for the given account,
	// oldest first. An empty slice is returned when none exist; it is not an
	// error.
	ListByAccountID(ctx context.Context, request ListAuditEntriesByAccountIDRequest) (*ListAuditEntriesByAccountIDResponse, error)

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
	// UserID, when non-empty, scopes the listing to the owning user so only
	// that user's audit entries are returned. Leave empty for internal/admin
	// listings that must see every entry.
	UserID string
}

// ListAuditEntriesResponse is the response for List.
type ListAuditEntriesResponse struct {
	AuditEntries []AuditEntry
}

// ListAuditEntriesByTransactionRefRequest is the request for ListByTransactionRef.
type ListAuditEntriesByTransactionRefRequest struct {
	TransactionRef string

	// UserID, when non-empty, scopes the listing to the owning user so entries
	// for another user's account are not returned. Leave empty for
	// internal/admin listings.
	UserID string
}

// ListAuditEntriesByTransactionRefResponse is the response for ListByTransactionRef.
type ListAuditEntriesByTransactionRefResponse struct {
	AuditEntries []AuditEntry
}

// ListAuditEntriesByAccountIDRequest is the request for ListByAccountID.
type ListAuditEntriesByAccountIDRequest struct {
	AccountID string

	// UserID, when non-empty, scopes the listing to the owning user so a caller
	// cannot read audit entries for an account they do not own. Leave empty for
	// internal/admin listings.
	UserID string
}

// ListAuditEntriesByAccountIDResponse is the response for ListByAccountID.
type ListAuditEntriesByAccountIDResponse struct {
	AuditEntries []AuditEntry
}

// GetAuditEntryByIDRequest is the request for GetByID.
type GetAuditEntryByIDRequest struct {
	ID int64

	// UserID, when non-empty, scopes the lookup to the owning user: an entry
	// owned by another user is reported as errs.ErrNotFound. Leave empty for
	// internal/admin lookups that must read any entry.
	UserID string
}

// GetAuditEntryByIDResponse is the response for GetByID.
type GetAuditEntryByIDResponse struct {
	AuditEntry AuditEntry
}

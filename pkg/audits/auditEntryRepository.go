package audits

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

type CreateAuditEntryRequest struct {
	AuditEntry AuditEntry
}

type CreateAuditEntryResponse struct {
	AuditEntry AuditEntry
}

type ListAuditEntriesRequest struct {
	// UserID names the owner the listing is scoped to. A caller granted
	// audit:read:all reads across owners regardless; otherwise only this user's
	// audit entries are returned.
	UserID string
}

type ListAuditEntriesResponse struct {
	AuditEntries []AuditEntry
}

type ListAuditEntriesByTransactionRefRequest struct {
	TransactionRef string

	// UserID names the owner the listing is scoped to. A caller granted
	// audit:read:all reads across owners regardless; otherwise entries for
	// another user's account are not returned.
	UserID string
}

type ListAuditEntriesByTransactionRefResponse struct {
	AuditEntries []AuditEntry
}

type ListAuditEntriesByAccountIDRequest struct {
	AccountID string

	// UserID names the owner the listing is scoped to. A caller granted
	// audit:read:all reads across owners regardless; otherwise a caller cannot
	// read audit entries for an account they do not own.
	UserID string
}

type ListAuditEntriesByAccountIDResponse struct {
	AuditEntries []AuditEntry
}

type GetAuditEntryByIDRequest struct {
	ID int64

	// UserID names the owner the lookup is scoped to. A caller granted
	// audit:read:all reads any entry; otherwise an entry owned by another user is
	// reported as errs.ErrNotFound.
	UserID string
}

type GetAuditEntryByIDResponse struct {
	AuditEntry AuditEntry
}

package audits

import (
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
)

// AuditServiceJSONRPCAdaptor exposes the audit trail of a transaction over
// JSON-RPC. It is a protected service callable by both members and admins: the
// authorization middleware places the verified login claim in the request
// context, and the listing is scoped to the caller's user id taken from that
// claim. A member sees only their own attempts; an admin holding audit:read:all
// sees every owner's. The repository enforces that scope, so a member querying a
// ref recorded against another owner's account simply gets an empty trail.
type AuditServiceJSONRPCAdaptor struct {
	audit AuditService
}

func NewAuditServiceJSONRPCAdaptor(audit AuditService) *AuditServiceJSONRPCAdaptor {
	return &AuditServiceJSONRPCAdaptor{audit: audit}
}

func (a *AuditServiceJSONRPCAdaptor) Name() string {
	return "AuditService"
}

// ListByTransactionRefParams is the wire request for ListByTransactionRef.
type ListByTransactionRefParams struct {
	TransactionRef string `json:"transaction_ref"`
}

// AuditEntryResult is the wire representation of one audit entry. The nullable
// fields stay pointers so a JSON null is emitted when the attempt could not
// resolve them (e.g. a malformed row with no account).
type AuditEntryResult struct {
	ID             int64     `json:"id"`
	UserID         string    `json:"user_id"`
	TransactionRef *string   `json:"transaction_ref"`
	AccountID      *string   `json:"account_id"`
	OwnerID        *string   `json:"owner_id"`
	Kind           *string   `json:"kind"`
	Points         *int64    `json:"points"`
	Outcome        string    `json:"outcome"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

// ListByTransactionRefResult is the wire response for ListByTransactionRef:
// every recorded attempt for the ref, oldest first. An empty slice is a valid
// result (the ref has no entries the caller may see), not an error.
type ListByTransactionRefResult struct {
	TransactionRef string             `json:"transaction_ref"`
	Entries        []AuditEntryResult `json:"entries"`
}

// FetchTransactionAuditTrail returns every audit entry recorded for a transaction
// ref, scoped to the caller. The UserID is taken from the verified claim — never
// the wire — so the repository's ownership scoping pins a member to their own
// entries.
func (a *AuditServiceJSONRPCAdaptor) FetchTransactionAuditTrail(r *http.Request, params *ListByTransactionRefParams, result *ListByTransactionRefResult) error {
	ctx := r.Context()

	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		log.Ctx(ctx).Error().Msg("audit: no login claim in context for protected method")
		return errs.ErrUnauthorized
	}

	resp, err := a.audit.FetchTransactionAuditTrail(ctx, ListAuditByRefRequest{
		TransactionRef: params.TransactionRef,
		UserID:         claim.UserID,
	})
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Str("transactionRef", params.TransactionRef).Msg("audit: trail lookup failed")
		if errors.Is(err, errs.ErrInvalidArgument) {
			return err
		}
		return errs.WithMessage(errs.ErrInternal, "could not retrieve audit trail")
	}

	result.TransactionRef = params.TransactionRef
	result.Entries = make([]AuditEntryResult, 0, len(resp.AuditEntries))
	for _, entry := range resp.AuditEntries {
		result.Entries = append(result.Entries, AuditEntryResult{
			ID:             entry.ID,
			UserID:         entry.UserID,
			TransactionRef: entry.TransactionRef,
			AccountID:      entry.AccountID,
			OwnerID:        entry.OwnerID,
			Kind:           entry.Kind,
			Points:         entry.Points,
			Outcome:        string(entry.Outcome),
			Reason:         entry.Reason,
			CreatedAt:      entry.CreatedAt,
		})
	}
	return nil
}

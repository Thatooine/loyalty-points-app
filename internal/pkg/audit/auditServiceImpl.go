package audit

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
)

// AuditServiceImpl fronts AuditEntryRepository for the audit-trail read path. It
// validates the request and delegates 1:1 to the repository, deliberately
// holding no logic of its own: the wire layer depends on this service rather
// than on the persistence port.
type AuditServiceImpl struct {
	auditEntries pkgAudit.AuditEntryRepository
}

func NewAuditServiceImpl(auditEntries pkgAudit.AuditEntryRepository) *AuditServiceImpl {
	return &AuditServiceImpl{auditEntries: auditEntries}
}

func (s *AuditServiceImpl) ListByTransactionRef(ctx context.Context, request pkgAudit.ListAuditByRefRequest) (*pkgAudit.ListAuditByRefResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for ListByTransactionRef: %w", err)
	}

	resp, err := s.auditEntries.ListByTransactionRef(ctx, pkgAudit.ListAuditEntriesByTransactionRefRequest{
		TransactionRef: request.TransactionRef,
		UserID:         request.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list audit entries by transaction ref: %w", err)
	}

	return &pkgAudit.ListAuditByRefResponse{AuditEntries: resp.AuditEntries}, nil
}

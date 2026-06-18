package audit

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	pkgAudit "github.com/Thatooine/loyalty-points-app/pkg/audits"
)

type AuditServiceImpl struct {
	auditEntryRepository pkgAudit.AuditEntryRepository
}

func NewAuditServiceImpl(auditEntries pkgAudit.AuditEntryRepository) *AuditServiceImpl {
	return &AuditServiceImpl{auditEntryRepository: auditEntries}
}

func (s *AuditServiceImpl) FetchTransactionAuditTrail(ctx context.Context, request pkgAudit.ListAuditByRefRequest) (*pkgAudit.ListAuditByRefResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for FetchTransactionAuditTrail: %w", err)
	}

	listByTransactionRefResponse, err := s.auditEntryRepository.ListByTransactionRef(
		ctx,
		pkgAudit.ListAuditEntriesByTransactionRefRequest{
			TransactionRef: request.TransactionRef,
			UserID:         request.UserID,
		})
	if err != nil {
		return nil, fmt.Errorf("could not list audit entries by transaction ref: %w", err)
	}

	return &pkgAudit.ListAuditByRefResponse{AuditEntries: listByTransactionRefResponse.AuditEntries}, nil
}

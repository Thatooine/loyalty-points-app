-- The audit trail no longer records where an attempt came from, so the source
-- column is dropped along with the AuditEntry.Source field.
ALTER TABLE audit_log DROP COLUMN source;

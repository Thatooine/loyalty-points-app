-- Rename audit_log.actor to user_id to match the renamed AuditEntry.UserID field.
ALTER TABLE audit_log RENAME COLUMN actor TO user_id;

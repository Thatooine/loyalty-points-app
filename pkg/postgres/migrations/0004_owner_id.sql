-- Denormalise the owning user onto the ledger and the audit trail. The owner is
-- the account's user (accounts.user_id); existing rows are backfilled from it.

-- transactions.owner_id is NOT NULL: a ledger entry always belongs to an
-- existing account, which always has an owner. Add it nullable, backfill, then
-- tighten the constraint so the backfill cannot fail mid-flight.
ALTER TABLE transactions ADD COLUMN owner_id TEXT REFERENCES users(id);
UPDATE transactions t SET owner_id = a.user_id FROM accounts a WHERE a.id = t.account_id;
ALTER TABLE transactions ALTER COLUMN owner_id SET NOT NULL;
CREATE INDEX idx_transactions_owner ON transactions(owner_id);

-- audit_log.owner_id is nullable, mirroring account_id: a rejected attempt for
-- an unknown account has no owner to record.
ALTER TABLE audit_log ADD COLUMN owner_id TEXT;
UPDATE audit_log al SET owner_id = a.user_id FROM accounts a WHERE a.id = al.account_id;

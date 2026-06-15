-- Back keyset pagination of the transaction ledger. List orders by
-- (recorded_at DESC, ref) and seeks from an opaque cursor; index that access
-- path for both scopes:
--   * owner-scoped listings use (owner_id, recorded_at DESC, ref);
--   * admin all-scope listings use (recorded_at DESC, ref).
-- The owner-prefixed composite covers any owner_id-only lookup as its leftmost
-- column, so the single-column idx_transactions_owner is now redundant.
DROP INDEX idx_transactions_owner;
CREATE INDEX idx_transactions_owner_keyset ON transactions (owner_id, recorded_at DESC, ref);
CREATE INDEX idx_transactions_keyset ON transactions (recorded_at DESC, ref);

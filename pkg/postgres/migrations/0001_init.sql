CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,                       -- bcrypt; set at create/bootstrap
    role          TEXT NOT NULL DEFAULT 'member'
                  CHECK (role IN ('member', 'admin')),
    created_at    TEXT NOT NULL,                       -- RFC3339 UTC
    token_version BIGINT NOT NULL DEFAULT 0            -- session epoch; bumped on logout to revoke all outstanding tokens
);

CREATE TABLE accounts (
    id         TEXT PRIMARY KEY,                        -- UUID assigned at persistence
    owner_id   TEXT NOT NULL REFERENCES users(id),      -- owning user (Account.OwnerID)
    name       TEXT NOT NULL,
    balance    BIGINT NOT NULL DEFAULT 0
               CHECK (balance >= 0),                    -- DB-level backstop for the overdraft rule
    created_at TEXT NOT NULL                            -- RFC3339 UTC
);
CREATE INDEX idx_accounts_owner ON accounts(owner_id);

CREATE TABLE transactions (
    id          TEXT PRIMARY KEY,                       -- UUID assigned at persistence
    ref         TEXT NOT NULL UNIQUE,                   -- idempotency key: the UNIQUE constraint IS the dedupe mechanism
    account_id  TEXT NOT NULL REFERENCES accounts(id),
    owner_id    TEXT NOT NULL REFERENCES users(id),     -- denormalised account owner, so entries attribute without a join
    kind        TEXT NOT NULL CHECK (kind IN ('earn', 'spend')),
    points      BIGINT NOT NULL,                        -- signed delta as applied: earn=+n, spend=-n
    occurred_at TEXT NOT NULL,                          -- business timestamp from the caller
    recorded_at TEXT NOT NULL,                          -- server timestamp
    created_by  TEXT NOT NULL                           -- acting principal (member or admin user id)
);
CREATE INDEX idx_transactions_account ON transactions(account_id, recorded_at);
-- Keyset pagination orders by (recorded_at DESC, ref) and seeks from an opaque
-- cursor. The owner-prefixed composite also serves any owner_id-only lookup as
-- its leftmost column, so no separate single-column owner index is needed.
CREATE INDEX idx_transactions_owner_keyset ON transactions (owner_id, recorded_at DESC, ref);
CREATE INDEX idx_transactions_keyset ON transactions (recorded_at DESC, ref);

CREATE TABLE audit_entries (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_ref TEXT,                               -- nullable: a malformed row may not have one (AuditEntry.TransactionRef)
    account_id      TEXT,                               -- nullable; no FK: a rejected attempt may name an unknown account
    owner_id        TEXT,                               -- nullable: an unknown account has no owner to record
    kind            TEXT,
    points          BIGINT,
    outcome         TEXT NOT NULL CHECK (outcome IN ('accepted', 'rejected', 'duplicate')),
    reason          TEXT NOT NULL,                      -- 'ok' | 'insufficient balance' | 'unknown account' | ...
    user_id         TEXT NOT NULL,                      -- principal that submitted the attempt
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_audit_entries_transaction_ref ON audit_entries(transaction_ref);

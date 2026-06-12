CREATE TABLE accounts (
    account_id    TEXT PRIMARY KEY,              -- caller-supplied natural key, e.g. "member-123"
    name          TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'member'
                  CHECK (role IN ('member', 'admin')),
    password_hash TEXT NOT NULL,                 -- bcrypt; set at create/bootstrap
    balance       INTEGER NOT NULL DEFAULT 0
                  CHECK (balance >= 0),          -- DB-level backstop for the overdraft rule
    created_at    TEXT NOT NULL                  -- RFC3339 UTC
);

CREATE TABLE transactions (
    ref         TEXT PRIMARY KEY,                -- idempotency key: the UNIQUE constraint IS the dedupe mechanism
    account_id  TEXT NOT NULL REFERENCES accounts(account_id),
    kind        TEXT NOT NULL CHECK (kind IN ('earn', 'spend', 'adjust')),
    points      INTEGER NOT NULL,                -- signed delta as applied: earn=+n, spend=-n, adjust=±n
    occurred_at TEXT NOT NULL,                   -- business timestamp from the caller
    recorded_at TEXT NOT NULL,                   -- server timestamp
    created_by  TEXT NOT NULL                    -- acting principal (member id or admin id)
);
CREATE INDEX idx_transactions_account ON transactions(account_id, recorded_at);

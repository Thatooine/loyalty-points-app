CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,                 -- bcrypt; set at create/bootstrap
    role          TEXT NOT NULL DEFAULT 'member'
                  CHECK (role IN ('member', 'admin')),
    created_at    TEXT NOT NULL                  -- RFC3339 UTC
);

CREATE TABLE accounts (
    id            TEXT PRIMARY KEY,              -- UUID assigned at persistence
    user_id       TEXT NOT NULL REFERENCES users(id),
    name          TEXT NOT NULL,
    balance       INTEGER NOT NULL DEFAULT 0
                  CHECK (balance >= 0),          -- DB-level backstop for the overdraft rule
    created_at    TEXT NOT NULL                  -- RFC3339 UTC
);
CREATE INDEX idx_accounts_user ON accounts(user_id);

CREATE TABLE transactions (
    id          TEXT PRIMARY KEY,                -- UUID assigned at persistence
    ref         TEXT NOT NULL UNIQUE,            -- idempotency key: the UNIQUE constraint IS the dedupe mechanism
    account_id  TEXT NOT NULL REFERENCES accounts(id),
    kind        TEXT NOT NULL CHECK (kind IN ('earn', 'spend', 'adjust')),
    points      INTEGER NOT NULL,                -- signed delta as applied: earn=+n, spend=-n, adjust=±n
    occurred_at TEXT NOT NULL,                   -- business timestamp from the caller
    recorded_at TEXT NOT NULL,                   -- server timestamp
    created_by  TEXT NOT NULL                    -- acting principal (member or admin user id)
);
CREATE INDEX idx_transactions_account ON transactions(account_id, recorded_at);

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ref         TEXT,                            -- nullable: malformed rows may not have one
    account_id  TEXT,
    kind        TEXT,
    points      INTEGER,
    source      TEXT NOT NULL,                   -- 'api' | 'batch:<filename>' | 'admin'
    outcome     TEXT NOT NULL CHECK (outcome IN ('accepted', 'rejected', 'duplicate')),
    reason      TEXT NOT NULL,                   -- 'ok' | 'insufficient balance' | 'unknown account' | ...
    actor       TEXT NOT NULL,
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_audit_ref ON audit_log(ref);

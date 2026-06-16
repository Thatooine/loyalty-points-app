-- token_version is the per-user session epoch. It is stamped into every issued
-- access token's claim and re-checked on each protected request: a token whose
-- stamped version is behind the user's current version is rejected. Bumping the
-- version (on logout / "log out everywhere") invalidates all of a user's
-- outstanding tokens at once. Existing rows default to 0, matching the zero
-- value stamped into tokens already in the wild.
ALTER TABLE users ADD COLUMN token_version BIGINT NOT NULL DEFAULT 0;

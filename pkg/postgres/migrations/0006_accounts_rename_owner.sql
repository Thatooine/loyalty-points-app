-- Rename accounts.user_id to owner_id to match the Account.OwnerID field and the
-- ownership-scoping the repository now enforces in SQL. The rename preserves the
-- foreign key and existing index; the index is renamed to match.
ALTER TABLE accounts RENAME COLUMN user_id TO owner_id;
ALTER INDEX idx_accounts_user RENAME TO idx_accounts_owner;

package sqlite

import (
	"errors"

	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"
)

// IsUniqueConstraintViolation reports whether err is a SQLite unique or
// primary-key constraint violation. Repositories use this to translate
// driver errors into domain sentinels (e.g. errs.ErrDuplicateRef).
func IsUniqueConstraintViolation(err error) bool {
	var sqliteErr *sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code()
	return code == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE ||
		code == sqlite3lib.SQLITE_CONSTRAINT_PRIMARYKEY
}

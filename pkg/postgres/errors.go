package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres SQLSTATE codes. The repository layer matches on these to translate
// driver errors into domain sentinels (e.g. errs.ErrDuplicateRef).
const (
	sqlStateUniqueViolation     = "23505"
	sqlStateForeignKeyViolation = "23503"
)

// IsUniqueConstraintViolation reports whether err is a Postgres unique or
// primary-key constraint violation. Repositories use this to translate driver
// errors into domain sentinels (e.g. errs.ErrDuplicateRef).
func IsUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == sqlStateUniqueViolation
}

// IsForeignKeyConstraintViolation reports whether err is a Postgres foreign-key
// constraint violation — e.g. a ledger row referencing an account that does not
// exist. Repositories translate it into errs.ErrNotFound.
func IsForeignKeyConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == sqlStateForeignKeyViolation
}

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// UserRepositoryImpl is the Postgres implementation of users.UserRepository.
// Every method resolves its executor from the context, so it runs inside an
// ambient transaction when one is present and against the pool otherwise.
type UserRepositoryImpl struct {
	db *sql.DB
}

func NewUserRepositoryImpl(db *sql.DB) *UserRepositoryImpl {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, request pkgUsers.CreateUserRequest) (*pkgUsers.CreateUserResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for Create: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	user := request.User
	_, err := exec.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, role, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		user.ID,
		user.Email,
		user.PasswordHash,
		string(user.Role),
		time.FormatTime(user.CreatedAt),
	)
	if err != nil {
		if postgres.IsUniqueConstraintViolation(err) {
			return nil, fmt.Errorf("user %s: %w", user.ID, errs.ErrAlreadyExists)
		}
		return nil, fmt.Errorf("could not insert user: %w", err)
	}

	return &pkgUsers.CreateUserResponse{User: user}, nil
}

func (r *UserRepositoryImpl) List(ctx context.Context, request pkgUsers.ListUsersRequest) (*pkgUsers.ListUsersResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for List: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping on the id column: holding user:read:all lists every user;
	// otherwise the WHERE clause restricts the listing to the caller's own record.
	query := `SELECT id, email, password_hash, role, created_at, token_version
		 FROM users`
	var args []any
	if !authorization.IsGranted(ctx, authorization.PermUserReadAll) {
		query += ` WHERE id = $1`
		args = append(args, request.UserID)
	}
	query += ` ORDER BY created_at, id`

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query users: %w", err)
	}
	defer rows.Close()

	var users []pkgUsers.User
	for rows.Next() {
		user, err := scanUser(rows.Scan)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate users: %w", err)
	}

	return &pkgUsers.ListUsersResponse{Users: users}, nil
}

func (r *UserRepositoryImpl) GetByID(ctx context.Context, request pkgUsers.GetUserByIDRequest) (*pkgUsers.GetUserByIDResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetByID: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping on the id column: unless the caller holds user:read:all
	// the lookup also requires id == caller, so a non-owner gets the same
	// ErrNotFound as for a missing user.
	query := `SELECT id, email, password_hash, role, created_at, token_version
		 FROM users
		 WHERE id = $1`
	args := []any{request.ID}
	if !authorization.IsGranted(ctx, authorization.PermUserReadAll) {
		query += ` AND id = $2`
		args = append(args, request.UserID)
	}

	row := exec.QueryRowContext(ctx, query, args...)

	user, err := scanUser(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user %s: %w", request.ID, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgUsers.GetUserByIDResponse{User: *user}, nil
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, request pkgUsers.GetUserByEmailRequest) (*pkgUsers.GetUserByEmailResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetByEmail: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	// Ownership scoping on the id column, mirroring GetByID. The login flow has
	// no claim yet, so IsGranted returns false and the scope branch is entered;
	// it passes SystemUserID, which is exempt so the lookup still resolves any
	// email. A caller holding user:read:all is likewise unscoped; everyone else
	// is restricted to their own record.
	query := `SELECT id, email, password_hash, role, created_at, token_version
		 FROM users
		 WHERE email = $1`
	args := []any{request.Email}
	if !authorization.IsGranted(ctx, authorization.PermUserReadAll) && request.UserID != pkgUsers.RootUserID {
		query += ` AND id = $2`
		args = append(args, request.UserID)
	}

	row := exec.QueryRowContext(ctx, query, args...)

	user, err := scanUser(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user %s: %w", request.Email, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgUsers.GetUserByEmailResponse{User: *user}, nil
}

// GetTokenVersion reads the user's current token_version. It is deliberately
// unscoped: it runs on the token-validation hot path before any login claim
// exists on the context, and the user id comes from a signature-verified token,
// not client input. A missing user is errs.ErrNotFound so a token for a deleted
// user is rejected.
func (r *UserRepositoryImpl) GetTokenVersion(ctx context.Context, request pkgUsers.GetTokenVersionRequest) (*pkgUsers.GetTokenVersionResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for GetTokenVersion: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	var version int64
	err := exec.QueryRowContext(ctx,
		`SELECT token_version FROM users WHERE id = $1`,
		request.UserID,
	).Scan(&version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user %s: %w", request.UserID, errs.ErrNotFound)
		}
		return nil, fmt.Errorf("could not read token version: %w", err)
	}

	return &pkgUsers.GetTokenVersionResponse{TokenVersion: version}, nil
}

// IncrementTokenVersion bumps the user's token_version by one in a single
// guarded UPDATE and returns the new value via RETURNING, so the read and write
// are atomic. Zero rows (no RETURNING row) means no such user — errs.ErrNotFound.
// Unscoped for the same reason as GetTokenVersion: the user id identifies the
// authenticated principal logging themselves out.
func (r *UserRepositoryImpl) IncrementTokenVersion(ctx context.Context, request pkgUsers.IncrementTokenVersionRequest) (*pkgUsers.IncrementTokenVersionResponse, error) {
	if err := request.Validate(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("request validation failed")
		return nil, fmt.Errorf("invalid request for IncrementTokenVersion: %w", err)
	}

	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	var version int64
	err := exec.QueryRowContext(ctx,
		`UPDATE users SET token_version = token_version + 1 WHERE id = $1
		 RETURNING token_version`,
		request.UserID,
	).Scan(&version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user %s: %w", request.UserID, errs.ErrNotFound)
		}
		return nil, fmt.Errorf("could not increment token version: %w", err)
	}

	return &pkgUsers.IncrementTokenVersionResponse{TokenVersion: version}, nil
}

func scanUser(scan func(dest ...any) error) (*pkgUsers.User, error) {
	var user pkgUsers.User
	var role, createdAt string

	if err := scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&role,
		&createdAt,
		&user.TokenVersion,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("could not scan user: %w", err)
	}

	user.Role = pkgUsers.Role(role)

	parsedCreatedAt, err := time.ParseTime(createdAt)
	if err != nil {
		return nil, err
	}
	user.CreatedAt = parsedCreatedAt

	return &user, nil
}

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgSQL "github.com/Thatooine/loyalty-points-app/pkg/sql"
	"github.com/Thatooine/loyalty-points-app/pkg/sqlite"
	"github.com/Thatooine/loyalty-points-app/pkg/time"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// UserRepositoryImpl is the SQLite implementation of users.UserRepository.
// Every method resolves its executor from the context, so it runs inside an
// ambient transaction when one is present and against the pool otherwise.
type UserRepositoryImpl struct {
	db *sql.DB
}

func NewUserRepositoryImpl(db *sql.DB) *UserRepositoryImpl {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, request pkgUsers.CreateUserRequest) (*pkgUsers.CreateUserResponse, error) {
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	user := request.User
	_, err := exec.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, role, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		user.ID,
		user.Email,
		user.PasswordHash,
		string(user.Role),
		time.FormatTime(user.CreatedAt),
	)
	if err != nil {
		if sqlite.IsUniqueConstraintViolation(err) {
			return nil, fmt.Errorf("user %s: %w", user.ID, errs.ErrAlreadyExists)
		}
		return nil, fmt.Errorf("could not insert user: %w", err)
	}

	return &pkgUsers.CreateUserResponse{User: user}, nil
}

func (r *UserRepositoryImpl) List(ctx context.Context, request pkgUsers.ListUsersRequest) (*pkgUsers.ListUsersResponse, error) {
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	rows, err := exec.QueryContext(ctx,
		`SELECT id, email, password_hash, role, created_at
		 FROM users
		 ORDER BY created_at, id`,
	)
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
	exec := pkgSQL.ExecutorFromContext(ctx, r.db)

	row := exec.QueryRowContext(ctx,
		`SELECT id, email, password_hash, role, created_at
		 FROM users
		 WHERE id = ?`,
		request.ID,
	)

	user, err := scanUser(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user %s: %w", request.ID, errs.ErrNotFound)
		}
		return nil, err
	}

	return &pkgUsers.GetUserByIDResponse{User: *user}, nil
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

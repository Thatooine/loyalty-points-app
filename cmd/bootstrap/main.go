// Command bootstrap wipes every data table and recreates the single well-known
// system user (fixed id, email, and bcrypt-hashed password) used by the
// email/password authentication flow.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgTime "github.com/Thatooine/loyalty-points-app/pkg/time"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// defaultPostgresDSN mirrors cmd/app's default so bootstrap works out of the box
// against the docker-compose container. Used only in the local environment.
const defaultPostgresDSN = "postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable"

const systemUserEmail = "system@mail.com"

// envLocal is the default environment. Outside local, bootstrap refuses to run
// with the baked dev credentials and requires them via env vars (fail closed).
const envLocal = "local"

// devSystemUserPassword is the well-known admin password used ONLY in the local
// environment. Any other environment must supply BOOTSTRAP_ADMIN_PASSWORD.
const devSystemUserPassword = "admin-user-123"

// schema_migrations is deliberately excluded so the schema (and applied-migration
// record) is preserved across a wipe.
var dataTables = []string{"transactions", "audit_entries", "accounts", "users"}

func main() {
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).With().Timestamp().Logger()

	if err := run(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("bootstrap failed")
	}
	log.Info().Msg("bootstrap complete")
}

func run(ctx context.Context) error {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = envLocal
	}

	dsn := os.Getenv("POSTGRES_DSN")
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")

	// Local gets the baked dev defaults; any real environment must supply its own
	// DSN and admin password or bootstrap refuses to run.
	if env == envLocal {
		if dsn == "" {
			dsn = defaultPostgresDSN
		}
		if password == "" {
			password = devSystemUserPassword
		}
	} else {
		if dsn == "" {
			return fmt.Errorf("POSTGRES_DSN is required outside the local environment")
		}
		if password == "" {
			return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD is required outside the local environment")
		}
	}

	db, err := postgres.NewClient(ctx, dsn)
	if err != nil {
		return fmt.Errorf("could not create postgres client: %w", err)
	}
	defer db.Close()

	// Apply migrations first so the tables we are about to wipe (and insert into)
	// are guaranteed to exist on a fresh database.
	if err := postgres.Migrate(ctx, db); err != nil {
		return fmt.Errorf("could not migrate database: %w", err)
	}

	if err := wipeTables(ctx, db); err != nil {
		return fmt.Errorf("could not wipe tables: %w", err)
	}
	log.Info().Strs("tables", dataTables).Msg("wiped all data tables")

	if err := createSystemUser(ctx, db, password); err != nil {
		return fmt.Errorf("could not create system user: %w", err)
	}
	// Password is a fixed well-known constant, so it is deliberately not logged.
	log.Info().
		Str("id", pkgUsers.RootUserID).
		Str("email", systemUserEmail).
		Msg("created system user")

	return nil
}

func wipeTables(ctx context.Context, db *sql.DB) error {
	stmt := fmt.Sprintf("TRUNCATE %s RESTART IDENTITY CASCADE",
		strings.Join(dataTables, ", "))
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("could not truncate tables: %w", err)
	}
	return nil
}

// createSystemUser writes the row with direct SQL rather than the user
// repository so it bypasses ownership-scoping and validation: bootstrap is
// operator tooling that owns the database.
func createSystemUser(ctx context.Context, db *sql.DB, password string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("could not hash password: %w", err)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, role, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		pkgUsers.RootUserID,
		systemUserEmail,
		string(passwordHash),
		string(pkgUsers.RoleAdmin),
		pkgTime.FormatTime(time.Now().UTC()),
	); err != nil {
		return fmt.Errorf("could not insert system user: %w", err)
	}
	return nil
}

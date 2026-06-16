// Command bootstrap resets the database to a known starting state: it wipes
// every data table and recreates the single well-known system user.
//
// The system user has a predictable, constant id (users.RootUserID) so it is
// the same on every run, and the email system@mail.com. Its password is the
// fixed, well-known systemUserPassword, stored bcrypt-hashed and printed once at
// the end. This is the principal resolved by GetByEmail in the email/password
// authentication flow (pkg/authentication/emailPasswordAuthenticator.go).
//
// Usage:
//
//	POSTGRES_DSN=... go run ./cmd/bootstrap
//
// The DSN defaults to the bundled docker-compose Postgres, matching cmd/app.
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
// against the docker-compose container.
const defaultPostgresDSN = "postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable"

// systemUserEmail is the fixed login email of the system principal. Its id is
// likewise fixed (users.RootUserID) so the principal is stable across every
// bootstrap run.
const systemUserEmail = "system@mail.com"

// systemUserPassword is the fixed system-user password. It is stored only as a
// bcrypt hash; this constant is the cleartext the bootstrap hashes and logs so
// the principal can be logged in with a known credential after a fresh run.
const systemUserPassword = "systemUser123"

// dataTables are the application data tables wiped on bootstrap. schema_migrations
// is deliberately excluded: the schema itself is preserved so migrations are not
// re-applied against already-existing tables.
var dataTables = []string{"transactions", "audit_log", "accounts", "users"}

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
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = defaultPostgresDSN
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

	password, err := createSystemUser(ctx, db)
	if err != nil {
		return fmt.Errorf("could not create system user: %w", err)
	}
	// The password is generated here and stored only as a bcrypt hash, so this
	// log line is the single chance to capture it. Print it explicitly.
	log.Info().
		Str("id", pkgUsers.RootUserID).
		Str("email", systemUserEmail).
		Str("password", password).
		Msg("created system user")

	return nil
}

// wipeTables truncates every data table in a single statement. CASCADE follows
// the foreign keys (transactions/accounts -> users), and RESTART IDENTITY resets
// audit_log's generated id sequence so a bootstrapped database is byte-for-byte
// reproducible.
func wipeTables(ctx context.Context, db *sql.DB) error {
	stmt := fmt.Sprintf("TRUNCATE %s RESTART IDENTITY CASCADE",
		strings.Join(dataTables, ", "))
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("could not truncate tables: %w", err)
	}
	return nil
}

// createSystemUser inserts the well-known system principal with a direct SQL
// INSERT rather than going through the user repository: bootstrap is operator
// tooling that owns the database, so it bypasses the repository's
// ownership-scoping and validation layers and writes the row itself. The id is
// the constant users.RootUserID; the password is the fixed systemUserPassword,
// returned to the caller for display and bcrypt-hashed before it touches the
// database; created_at is the RFC3339Nano UTC TEXT the schema expects. The role
// is admin so the system principal can perform operator actions.
func createSystemUser(ctx context.Context, db *sql.DB) (string, error) {
	password := systemUserPassword

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("could not hash password: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, role, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		pkgUsers.RootUserID,
		systemUserEmail,
		string(passwordHash),
		string(pkgUsers.RoleAdmin),
		pkgTime.FormatTime(time.Now().UTC()),
	)
	if err != nil {
		return "", fmt.Errorf("could not insert system user: %w", err)
	}
	return password, nil
}

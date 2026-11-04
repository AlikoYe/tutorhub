// Package dbtest gives integration tests a freshly migrated PostgreSQL schema.
// Tests are skipped unless TEST_DATABASE_URL is set.
package dbtest

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/AlikoYe/tutorhub/backend/migrations"
)

// URL returns TEST_DATABASE_URL or skips the test.
func URL(t testing.TB) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping database test")
	}
	return url
}

// Reset drops and recreates the public schema and returns a goose provider for
// it, without applying migrations. The database name must contain "test" so a
// dev database is never wiped by mistake.
func Reset(t testing.TB) (*sql.DB, *goose.Provider) {
	t.Helper()
	ctx := context.Background()

	db, err := sql.Open("pgx", URL(t))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var name string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	if !strings.Contains(name, "test") {
		t.Fatalf("refusing to wipe database %q: TEST_DATABASE_URL must point at a database whose name contains \"test\"", name)
	}
	if _, err := db.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatalf("goose provider: %v", err)
	}
	return db, provider
}

// New resets the schema, applies all migrations and returns a fresh pool.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	_, provider := Reset(t)
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), URL(t))
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

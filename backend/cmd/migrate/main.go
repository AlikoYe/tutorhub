// Command migrate applies the embedded goose migrations.
//
//	migrate up | down | status
//
// Reads DATABASE_URL from the environment.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/AlikoYe/tutorhub/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return errors.New("usage: migrate up|down|status")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return err
	}

	ctx := context.Background()
	switch os.Args[1] {
	case "up":
		results, err := provider.Up(ctx)
		for _, r := range results {
			fmt.Println(r)
		}
		if err == nil && len(results) == 0 {
			fmt.Println("no pending migrations")
		}
		return err
	case "down":
		r, err := provider.Down(ctx)
		if r != nil {
			fmt.Println(r)
		}
		return err
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			applied := "pending"
			if s.State == goose.StateApplied {
				applied = "applied " + s.AppliedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("%-40s %s\n", s.Source.Path, applied)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q (want up, down or status)", os.Args[1])
	}
}

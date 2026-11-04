package migrations_test

import (
	"context"
	"testing"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/dbtest"
)

// TestMigrationsUpDownUp checks every migration applies, rolls back and re-applies cleanly.
func TestMigrationsUpDownUp(t *testing.T) {
	_, provider := dbtest.Reset(t)
	ctx := context.Background()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("down to 0: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func TestSubjectSeed(t *testing.T) {
	db := dbtest.New(t)
	var n int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM subjects WHERE name_ru <> '' AND name_kk <> '' AND name_en <> ''`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 40 {
		t.Fatalf("%d seeded subjects, want at least 40", n)
	}
}

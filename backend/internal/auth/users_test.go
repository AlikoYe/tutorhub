package auth_test

import (
	"context"
	"testing"

	"github.com/AlikoYe/tutorhub/backend/internal/auth"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/dbtest"
)

func TestUserRepoUpsert(t *testing.T) {
	db := dbtest.New(t)
	ctx := context.Background()
	repo := auth.NewUserRepo(db)

	role, blocked, err := repo.Upsert(ctx, auth.User{ID: 5, FirstName: "Ann", Username: "ann", LanguageCode: "kk"})
	if err != nil || role != auth.RoleUser || blocked {
		t.Fatalf("first upsert: role=%q err=%v", role, err)
	}

	// Name and username are refreshed, role and a known language are kept.
	if _, err := db.Exec(ctx, `UPDATE users SET role = 'admin' WHERE telegram_id = 5`); err != nil {
		t.Fatal(err)
	}
	role, _, err = repo.Upsert(ctx, auth.User{ID: 5, FirstName: "Ann", LastName: "Lee"})
	if err != nil || role != auth.RoleAdmin {
		t.Fatalf("second upsert: role=%q err=%v", role, err)
	}

	var name, lang string
	var username *string
	if err := db.QueryRow(ctx, `SELECT name, username, language_code FROM users WHERE telegram_id = 5`).Scan(&name, &username, &lang); err != nil {
		t.Fatal(err)
	}
	if name != "Ann Lee" || username != nil || lang != "kk" {
		t.Errorf("stored name=%q username=%v lang=%q, want \"Ann Lee\", NULL, kk", name, username, lang)
	}
}

func TestEnsureAdmins(t *testing.T) {
	db := dbtest.New(t)
	ctx := context.Background()
	repo := auth.NewUserRepo(db)

	if _, _, err := repo.Upsert(ctx, auth.User{ID: 7, FirstName: "Existing"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureAdmins(ctx, []int64{7, 8}); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureAdmins(ctx, nil); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin' AND telegram_id IN (7, 8)`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("%d admins, want 2", n)
	}
	role, _, err := repo.Upsert(ctx, auth.User{ID: 8, FirstName: "New"})
	if err != nil || role != auth.RoleAdmin {
		t.Fatalf("admin after login: role=%q err=%v", role, err)
	}
}

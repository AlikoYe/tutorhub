package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Values of users.role.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// UserRepo is the PostgreSQL UserStore.
type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Upsert inserts the user or refreshes their name, username and language, and returns their
// role and blocked flag.
func (r *UserRepo) Upsert(ctx context.Context, u User) (string, bool, error) {
	var role string
	var blocked bool
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name, username, language_code)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE
		SET name = EXCLUDED.name, username = EXCLUDED.username,
		    language_code = COALESCE(EXCLUDED.language_code, users.language_code), updated_at = now()
		RETURNING role, blocked`,
		u.ID, u.DisplayName(), nullable(u.Username), nullable(u.LanguageCode)).Scan(&role, &blocked)
	if err != nil {
		return "", false, fmt.Errorf("upsert user: %w", err)
	}
	return role, blocked, nil
}

// SetBlocked blocks or unblocks a user; false when the user does not exist. Admins cannot be blocked.
func (r *UserRepo) SetBlocked(ctx context.Context, telegramID int64, blocked bool) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE users SET blocked = $2, updated_at = now() WHERE telegram_id = $1 AND role <> 'admin'`, telegramID, blocked)
	if err != nil {
		return false, fmt.Errorf("set blocked: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// EnsureAdmins creates or promotes the given Telegram ids to role admin.
func (r *UserRepo) EnsureAdmins(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (telegram_id, role) SELECT unnest($1::bigint[]), 'admin'
		ON CONFLICT (telegram_id) DO UPDATE SET role = 'admin', updated_at = now()`, ids)
	if err != nil {
		return fmt.Errorf("ensure admins: %w", err)
	}
	return nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

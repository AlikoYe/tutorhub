// Package subject serves the seeded subject taxonomy.
package subject

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
)

var ErrNotFound = errors.New("subject not found")

type Subject struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	NameRU   string `json:"name_ru"`
	NameKK   string `json:"name_kk"`
	NameEN   string `json:"name_en"`
	Category string `json:"category"`
}

// Repo reads subjects. There are no business rules, so handlers use it directly.
type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

const selectSubject = `SELECT id, slug, name_ru, name_kk, name_en, category FROM subjects`

func (r *Repo) List(ctx context.Context) ([]Subject, error) {
	rows, err := r.db.Query(ctx, selectSubject+` ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("select subjects: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Subject])
}

func (r *Repo) Get(ctx context.Context, id int64) (Subject, error) {
	rows, err := r.db.Query(ctx, selectSubject+` WHERE id = $1`, id)
	if err != nil {
		return Subject{}, fmt.Errorf("select subject: %w", err)
	}
	s, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[Subject])
	if errors.Is(err, pgx.ErrNoRows) {
		return Subject{}, ErrNotFound
	}
	return s, err
}

// ListHandler serves GET /api/subjects.
func ListHandler(repo *Repo, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := repo.List(r.Context())
		if err != nil {
			log.ErrorContext(r.Context(), "list subjects", slog.Any("err", err))
			httpx.Internal(w)
			return
		}
		httpx.JSON(w, http.StatusOK, items)
	}
}

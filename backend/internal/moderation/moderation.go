// Package moderation handles profile reports from users and blocking users.
package moderation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/auth"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
)

var ErrNotFound = errors.New("not found")

// Reasons a profile can be reported for.
var Reasons = []string{"fake", "inappropriate", "spam", "no_show", "other"}

type Report struct {
	ID         int64                    `json:"id"`
	Specialist specialist.SpecialistRef `json:"specialist"`
	Reporter   specialist.Owner         `json:"reporter"`
	Reason     string                   `json:"reason"`
	Text       string                   `json:"text"`
	Status     string                   `json:"status"`
	CreatedAt  time.Time                `json:"created_at"`
	ResolvedAt *time.Time               `json:"resolved_at"`
}

// Notifier tells admins about new reports. Implemented by bot.Notifier.
type Notifier interface {
	ProfileReported(ctx context.Context, adminIDs []int64, specialistName, reason, text string)
}

type Service struct {
	db       *pgxpool.Pool
	profiles *specialist.Service
	users    *auth.UserRepo
	notifier Notifier
	loc      *time.Location
}

func NewService(db *pgxpool.Pool, profiles *specialist.Service, users *auth.UserRepo, notifier Notifier, loc *time.Location) *Service {
	return &Service{db: db, profiles: profiles, users: users, notifier: notifier, loc: loc}
}

type invalidError struct{ msg string }

func (e *invalidError) Error() string { return e.msg }

const reportSelect = `SELECT rp.id, sp.id, sp.display_name, sp.photo_url, u.telegram_id, u.name, u.username,
	rp.reason, rp.text, rp.status, rp.created_at, rp.resolved_at
	FROM reports rp JOIN specialists sp ON sp.id = rp.specialist_id JOIN users u ON u.telegram_id = rp.reporter_id`

func (s *Service) list(ctx context.Context, where string, arg any) ([]Report, error) {
	rows, err := s.db.Query(ctx, reportSelect+` WHERE `+where+` ORDER BY rp.created_at DESC LIMIT 200`, arg)
	if err != nil {
		return nil, fmt.Errorf("select reports: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Report, error) {
		var x Report
		err := r.Scan(&x.ID, &x.Specialist.ID, &x.Specialist.DisplayName, &x.Specialist.PhotoURL,
			&x.Reporter.TelegramID, &x.Reporter.Name, &x.Reporter.Username, &x.Reason, &x.Text, &x.Status, &x.CreatedAt, &x.ResolvedAt)
		x.CreatedAt = x.CreatedAt.In(s.loc).Truncate(time.Second)
		if x.ResolvedAt != nil {
			t := x.ResolvedAt.In(s.loc).Truncate(time.Second)
			x.ResolvedAt = &t
		}
		return x, err
	})
}

// Report files a report about a visible profile; a repeat while one is open returns the open one.
func (s *Service) Report(ctx context.Context, v specialist.Viewer, specialistID int64, reason, text string) (Report, error) {
	text = strings.TrimSpace(text)
	valid := false
	for _, r := range Reasons {
		valid = valid || r == reason
	}
	if !valid {
		return Report{}, &invalidError{"reason must be one of " + strings.Join(Reasons, ", ")}
	}
	if utf8.RuneCountInString(text) > 1000 {
		return Report{}, &invalidError{"text must be at most 1000 characters"}
	}
	p, err := s.profiles.Profile(ctx, v, specialistID)
	if err != nil {
		return Report{}, err
	}
	var id int64
	err = s.db.QueryRow(ctx, `SELECT id FROM reports WHERE specialist_id = $1 AND reporter_id = $2 AND status = 'open'`, specialistID, v.UserID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.db.QueryRow(ctx, `INSERT INTO reports (specialist_id, reporter_id, reason, text) VALUES ($1, $2, $3, $4) RETURNING id`,
			specialistID, v.UserID, reason, text).Scan(&id)
		if err == nil {
			admins, aerr := s.profiles.Repo().AdminIDs(ctx)
			if aerr == nil {
				s.notifier.ProfileReported(ctx, admins, p.DisplayName, reason, text)
			}
		}
	}
	if err != nil {
		return Report{}, fmt.Errorf("report: %w", err)
	}
	list, err := s.list(ctx, `rp.id = $1`, id)
	if err != nil || len(list) == 0 {
		return Report{}, err
	}
	return list[0], nil
}

// Resolve closes a report as resolved or dismissed.
func (s *Service) Resolve(ctx context.Context, adminID, id int64, status string) (Report, error) {
	if status != "resolved" && status != "dismissed" {
		return Report{}, &invalidError{"status must be resolved or dismissed"}
	}
	tag, err := s.db.Exec(ctx, `UPDATE reports SET status = $2, resolved_at = now(), resolved_by = $3 WHERE id = $1`, id, status, adminID)
	if err != nil {
		return Report{}, fmt.Errorf("resolve report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Report{}, ErrNotFound
	}
	list, err := s.list(ctx, `rp.id = $1`, id)
	if err != nil || len(list) == 0 {
		return Report{}, err
	}
	return list[0], nil
}

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func viewer(r *http.Request) specialist.Viewer {
	u, _ := auth.CurrentUser(r)
	return specialist.Viewer{UserID: u.ID, IsAdmin: u.Role == auth.RoleAdmin}
}

type reportRequest struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

// ReportHandler serves POST /api/specialists/{id}/report.
func (h *Handler) ReportHandler(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var req reportRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	x, err := h.svc.Report(r.Context(), viewer(r), id, req.Reason, req.Text)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, x)
}

// AdminRoutes mounts under /api/admin.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/reports", h.reports)
	r.Post("/reports/{id}/resolve", h.resolve)
	r.Post("/users/{telegramID}/block", h.block(true))
	r.Post("/users/{telegramID}/unblock", h.block(false))
}

func (h *Handler) reports(w http.ResponseWriter, r *http.Request) {
	status, err := httpx.QueryEnum(r, "status", []string{"open", "resolved", "dismissed"})
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if status == "" {
		status = "open"
	}
	items, err := h.svc.list(r.Context(), `rp.status = $1`, status)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type resolveRequest struct {
	Status string `json:"status"`
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var req resolveRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	x, err := h.svc.Resolve(r.Context(), u.ID, id, req.Status)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, x)
}

func (h *Handler) block(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "telegramID"), 10, 64)
		if err != nil || id <= 0 {
			httpx.BadRequest(w, "telegram id must be a positive integer")
			return
		}
		ok, err := h.svc.users.SetBlocked(r.Context(), id, on)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		if !ok {
			httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, "user not found (admins cannot be blocked)")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var inv *invalidError
	switch {
	case errors.As(err, &inv):
		httpx.BadRequest(w, err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, specialist.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "moderation handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}

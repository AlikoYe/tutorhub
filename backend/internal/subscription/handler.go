package subscription

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/tutorhub/backend/internal/auth"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
)

type Handler struct {
	svc         *Service
	specialists *specialist.Repo
	log         *slog.Logger
}

func NewHandler(svc *Service, specialists *specialist.Repo, log *slog.Logger) *Handler {
	return &Handler{svc: svc, specialists: specialists, log: log}
}

// PlansHandler serves GET /api/plans.
func (h *Handler) PlansHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, h.svc.Plans())
}

// TeacherHandler serves GET /api/teacher/subscription.
func (h *Handler) TeacherHandler(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	st, err := h.svc.MyState(r.Context(), h.specialists, u.ID)
	h.respond(w, r, st, err)
}

// SpecialistRoutes mounts inside /api/admin/specialists (behind auth.RequireAdmin).
func (h *Handler) SpecialistRoutes(r chi.Router) {
	r.Get("/{id}/subscriptions", h.state)
	r.Post("/{id}/subscriptions", h.activate)
}

// AdminRoutes mounts under /api/admin (behind auth.RequireAdmin).
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Post("/subscriptions/{id}/end", h.end)
	r.Get("/subscriptions/expiring", h.expiring)
	r.Post("/founding-period", h.founding)
}

func (h *Handler) state(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	st, err := h.svc.State(r.Context(), id)
	h.respond(w, r, st, err)
}

type activateRequest struct {
	Plan   string `json:"plan"`
	Months int    `json:"months"`
	EndsOn string `json:"ends_on"`
	Note   string `json:"note"`
}

func parseDate(v, name string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	d, err := time.Parse(httpx.DateLayout, v)
	if err != nil {
		return nil, &InvalidError{name + " must be a date in YYYY-MM-DD format"}
	}
	return &d, nil
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var req activateRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	endsOn, err := parseDate(req.EndsOn, "ends_on")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	st, err := h.svc.Activate(r.Context(), id, ActivateInput{Plan: req.Plan, Months: req.Months, EndsOn: endsOn, Note: req.Note, AdminID: u.ID})
	h.respond(w, r, st, err)
}

func (h *Handler) end(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	st, err := h.svc.End(r.Context(), id)
	h.respond(w, r, st, err)
}

func (h *Handler) expiring(w http.ResponseWriter, r *http.Request) {
	days, err := httpx.QueryIntDefault(r, "days", 14, 1, 90)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	items, err := h.svc.Expiring(r.Context(), days)
	h.respond(w, r, items, err)
}

type foundingRequest struct {
	EndsOn string `json:"ends_on"`
}

func (h *Handler) founding(w http.ResponseWriter, r *http.Request) {
	var req foundingRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	d, err := parseDate(req.EndsOn, "ends_on")
	if err != nil || d == nil {
		httpx.BadRequest(w, "ends_on is required (YYYY-MM-DD)")
		return
	}
	n, err := h.svc.SetFoundingPeriod(r.Context(), *d)
	h.respond(w, r, map[string]int{"updated": n}, err)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	var inv *InvalidError
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusOK, v)
	case errors.As(err, &inv):
		httpx.BadRequest(w, err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, specialist.ErrNotFound), errors.Is(err, specialist.ErrNoProfile):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "subscription handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}

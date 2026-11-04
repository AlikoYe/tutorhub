package availability

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
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// TeacherRoutes mounts under /api/teacher/availability.
func (h *Handler) TeacherRoutes(r chi.Router) {
	r.Get("/", h.template)
	r.Put("/rules", h.setRules)
	r.Put("/days-off/{day}", h.dayOff(true))
	r.Delete("/days-off/{day}", h.dayOff(false))
}

func (h *Handler) template(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	t, err := h.svc.Template(r.Context(), u.ID)
	h.respond(w, r, t, err)
}

type rulesRequest struct {
	Rules []RuleJSON `json:"rules"`
}

func (h *Handler) setRules(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	var req rulesRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if req.Rules == nil {
		httpx.BadRequest(w, "rules is required (use [] to clear)")
		return
	}
	t, err := h.svc.SetRules(r.Context(), u.ID, req.Rules)
	h.respond(w, r, t, err)
}

func (h *Handler) dayOff(closed bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.CurrentUser(r)
		day, err := time.Parse(httpx.DateLayout, chi.URLParam(r, "day"))
		if err != nil {
			httpx.BadRequest(w, "day must be a date in YYYY-MM-DD format")
			return
		}
		t, err := h.svc.SetDayOff(r.Context(), u.ID, day, closed)
		h.respond(w, r, t, err)
	}
}

// Slots serves GET /api/specialists/{id}/slots.
func (h *Handler) Slots(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var subjectID int64
	if r.URL.Query().Get("subject_id") != "" {
		if subjectID, err = httpx.QueryID(r, "subject_id"); err != nil {
			httpx.BadRequest(w, err.Error())
			return
		}
	}
	var from *time.Time
	if r.URL.Query().Get("from") != "" {
		d, err := httpx.QueryDate(r, "from")
		if err != nil {
			httpx.BadRequest(w, err.Error())
			return
		}
		from = &d
	}
	days, err := httpx.QueryIntDefault(r, "days", 7, 1, MaxDaysAhead+1)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	slots, err := h.svc.FreeSlots(r.Context(), specialist.Viewer{UserID: u.ID, IsAdmin: u.Role == auth.RoleAdmin}, id, subjectID, from, days)
	h.respond(w, r, slots, err)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	var inv *InvalidError
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusOK, v)
	case errors.As(err, &inv), errors.Is(err, ErrUnknownSubject):
		httpx.BadRequest(w, err.Error())
	case errors.Is(err, specialist.ErrNotFound), errors.Is(err, specialist.ErrNoProfile):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "availability handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}

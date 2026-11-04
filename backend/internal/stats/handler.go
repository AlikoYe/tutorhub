package stats

import (
	"errors"
	"log/slog"
	"net/http"

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

// Teacher serves GET /api/teacher/stats.
func (h *Handler) Teacher(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	weeks, err := httpx.QueryIntDefault(r, "weeks", 8, 1, 52)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	id, err := h.specialists.IDByUser(r.Context(), u.ID)
	if errors.Is(err, specialist.ErrNoProfile) {
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
		return
	}
	if err != nil {
		h.internal(w, r, err)
		return
	}
	out, err := h.svc.TeacherStats(r.Context(), id, weeks)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// Dashboard serves GET /api/admin/dashboard.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	days, err := httpx.QueryIntDefault(r, "days", 14, 1, 90)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	out, err := h.svc.Dashboard(r.Context(), days)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) internal(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "stats handler", slog.Any("err", err))
	httpx.Internal(w)
}

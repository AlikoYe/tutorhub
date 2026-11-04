package review

import (
	"errors"
	"log/slog"
	"net/http"

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

// SpecialistRoutes mounts inside /api/specialists.
func (h *Handler) SpecialistRoutes(r chi.Router) {
	r.Get("/{id}/reviews", h.list)
	r.Get("/{id}/review", h.mine)
	r.Put("/{id}/review", h.save)
	r.Delete("/{id}/review", h.delete)
}

// AdminRoutes mounts under /api/admin/reviews.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/", h.adminList)
	r.Post("/{id}/hide", h.setStatus("hidden"))
	r.Post("/{id}/unhide", h.setStatus("published"))
}

func viewer(r *http.Request) specialist.Viewer {
	u, _ := auth.CurrentUser(r)
	return specialist.Viewer{UserID: u.ID, IsAdmin: u.Role == auth.RoleAdmin}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	items, err := h.svc.List(r.Context(), viewer(r), id)
	h.respond(w, r, items, err)
}

func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	m, err := h.svc.Mine(r.Context(), viewer(r), id)
	h.respond(w, r, m, err)
}

type saveRequest struct {
	Rating int    `json:"rating"`
	Text   string `json:"text"`
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var req saveRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	x, err := h.svc.Save(r.Context(), viewer(r), id, req.Rating, req.Text)
	h.respond(w, r, x, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.Delete(r.Context(), viewer(r).UserID, id); err != nil {
		h.respond(w, r, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) adminList(w http.ResponseWriter, r *http.Request) {
	status, err := httpx.QueryEnum(r, "status", []string{"published", "hidden"})
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if status == "" {
		status = "published"
	}
	items, err := h.svc.AdminList(r.Context(), status)
	h.respond(w, r, items, err)
}

func (h *Handler) setStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.BadRequest(w, err.Error())
			return
		}
		x, err := h.svc.SetStatus(r.Context(), id, status)
		h.respond(w, r, x, err)
	}
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	var inv *InvalidError
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusOK, v)
	case errors.As(err, &inv):
		httpx.BadRequest(w, err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, specialist.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	case errors.Is(err, ErrNoCompletedLesson):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeNoCompletedLesson, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "review handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}

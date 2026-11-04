package lesson

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/tutorhub/backend/internal/auth"
	"github.com/AlikoYe/tutorhub/backend/internal/availability"
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

// StudentRoutes mounts under /api/lessons.
func (h *Handler) StudentRoutes(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/my", h.mine)
	r.Post("/{id}/cancel", h.cancelAsStudent)
}

// TeacherRoutes mounts under /api/teacher.
func (h *Handler) TeacherRoutes(r chi.Router) {
	r.Get("/lessons", h.teacherList)
	r.Post("/lessons/{id}/confirm", h.decide(true))
	r.Post("/lessons/{id}/decline", h.decide(false))
	r.Post("/lessons/{id}/cancel", h.cancelAsTeacher)
	r.Get("/students", h.students)
	r.Put("/students/{studentID}/note", h.setNote)
}

type createRequest struct {
	SpecialistID int64     `json:"specialist_id"`
	SubjectID    int64     `json:"subject_id"`
	StartsAt     time.Time `json:"starts_at"`
	IsTrial      bool      `json:"is_trial"`
	Note         string    `json:"note"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	switch {
	case req.SpecialistID <= 0:
		httpx.BadRequest(w, "specialist_id is required")
		return
	case req.SubjectID <= 0:
		httpx.BadRequest(w, "subject_id is required")
		return
	case req.StartsAt.IsZero():
		httpx.BadRequest(w, "starts_at is required (RFC 3339)")
		return
	}
	l, err := h.svc.Create(r.Context(), CreateInput{
		StudentID: u.ID, SpecialistID: req.SpecialistID, SubjectID: req.SubjectID,
		StartsAt: req.StartsAt, IsTrial: req.IsTrial, Note: req.Note,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, l)
}

func scope(r *http.Request, allowed ...string) (string, error) {
	s, err := httpx.QueryEnum(r, "scope", allowed)
	if s == "" {
		s = ScopeUpcoming
	}
	return s, err
}

func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	sc, err := scope(r, ScopeUpcoming, ScopePast)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	items, err := h.svc.Mine(r.Context(), u.ID, sc)
	h.respond(w, r, items, err)
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

func readReason(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req reasonRequest
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			httpx.BadRequest(w, err.Error())
			return "", false
		}
	}
	return req.Reason, true
}

func (h *Handler) cancelAsStudent(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	reason, ok := readReason(w, r)
	if !ok {
		return
	}
	l, err := h.svc.CancelAsStudent(r.Context(), u.ID, id, reason)
	h.respond(w, r, l, err)
}

func (h *Handler) teacherList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	sc, err := scope(r, ScopeRequests, ScopeUpcoming, ScopePast)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	items, err := h.svc.ForTeacher(r.Context(), u.ID, sc)
	h.respond(w, r, items, err)
}

func (h *Handler) decide(confirm bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := auth.CurrentUser(r)
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.BadRequest(w, err.Error())
			return
		}
		l, err := h.svc.Decide(r.Context(), u.ID, id, confirm)
		h.respond(w, r, l, err)
	}
}

func (h *Handler) cancelAsTeacher(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	reason, ok := readReason(w, r)
	if !ok {
		return
	}
	l, err := h.svc.CancelAsTeacher(r.Context(), u.ID, id, reason)
	h.respond(w, r, l, err)
}

func (h *Handler) students(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	items, err := h.svc.Students(r.Context(), u.ID)
	h.respond(w, r, items, err)
}

type noteRequest struct {
	Note string `json:"note"`
}

func (h *Handler) setNote(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentID"), 10, 64)
	if err != nil || studentID <= 0 {
		httpx.BadRequest(w, "student id must be a positive integer")
		return
	}
	var req noteRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.SetNote(r.Context(), u.ID, studentID, req.Note); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNoteTooLong):
		httpx.BadRequest(w, err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrSpecialistGone), errors.Is(err, specialist.ErrNoProfile):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, httpx.CodeForbidden, err.Error())
	case errors.Is(err, ErrSlotTaken):
		httpx.Error(w, http.StatusConflict, httpx.CodeSlotTaken, err.Error())
	case errors.Is(err, ErrStudentBusy):
		httpx.Error(w, http.StatusConflict, httpx.CodeStudentBusy, err.Error())
	case errors.Is(err, ErrNotPending):
		httpx.Error(w, http.StatusConflict, httpx.CodeNotPending, err.Error())
	case errors.Is(err, ErrNotCancellable):
		httpx.Error(w, http.StatusConflict, httpx.CodeNotCancellable, err.Error())
	case errors.Is(err, ErrOwnProfile):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeOwnProfile, err.Error())
	case errors.Is(err, ErrSubjectNotOffered):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeSubjectNotOffered, err.Error())
	case errors.Is(err, ErrTrialUnavailable):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeTrialUnavailable, err.Error())
	case errors.Is(err, availability.ErrOutside), errors.Is(err, availability.ErrTooFarAhead):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeOutsideAvailability, err.Error())
	case errors.Is(err, availability.ErrTooSoon):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeTooSoon, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "lesson handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}

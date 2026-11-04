package specialist

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/tutorhub/backend/internal/auth"
	"github.com/AlikoYe/tutorhub/backend/internal/geo"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
	"github.com/AlikoYe/tutorhub/backend/internal/visibility"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// CatalogueRoutes mounts under /api/specialists.
func (h *Handler) CatalogueRoutes(r chi.Router) {
	r.Get("/", h.search)
	r.Get("/{id}", h.profile)
}

// MyRoutes mounts under /api/my/specialist.
func (h *Handler) MyRoutes(r chi.Router) {
	r.Get("/", h.my)
	r.Put("/", h.save)
	r.Post("/submit", h.submit)
	r.Post("/hide", h.hide)
	r.Post("/unhide", h.unhide)
	r.Post("/photo", h.photo)
}

// AdminRoutes mounts under /api/admin/specialists (behind auth.RequireAdmin).
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/", h.adminList)
	r.Get("/{id}", h.adminGet)
	r.Post("/{id}/approve", h.approve)
	r.Post("/{id}/reject", h.reject)
}

// ParseFilter reads catalogue filters from the query string.
func ParseFilter(r *http.Request) (Filter, error) {
	var f Filter
	var err error
	q := r.URL.Query()
	if q.Get("subject_id") != "" {
		if f.SubjectID, err = httpx.QueryID(r, "subject_id"); err != nil {
			return f, err
		}
	}
	if f.Kind, err = httpx.QueryEnum(r, "kind", geo.Kinds); err != nil {
		return f, err
	}
	if f.Level, err = httpx.QueryEnum(r, "level", geo.Levels); err != nil {
		return f, err
	}
	if f.Language, err = httpx.QueryEnum(r, "language", geo.Languages); err != nil {
		return f, err
	}
	if f.Format, err = httpx.QueryEnum(r, "format", geo.Formats); err != nil {
		return f, err
	}
	f.City, f.District = q.Get("city"), q.Get("district")
	if f.District != "" && !geo.ValidDistrict(orDefault(f.City, geo.DefaultCity), f.District) {
		return f, errors.New("unknown district")
	}
	if f.PriceMin, err = httpx.QueryOptionalInt(r, "price_min", 0, 1000000); err != nil {
		return f, err
	}
	if f.PriceMax, err = httpx.QueryOptionalInt(r, "price_max", 0, 1000000); err != nil {
		return f, err
	}
	if f.VerifiedOnly, err = httpx.QueryBool(r, "verified"); err != nil {
		return f, err
	}
	if f.Query = q.Get("q"); utf8.RuneCountInString(f.Query) > 100 {
		return f, errors.New("q must be at most 100 characters")
	}
	if f.Limit, err = httpx.QueryIntDefault(r, "limit", 20, 1, 50); err != nil {
		return f, err
	}
	if f.Offset, err = httpx.QueryIntDefault(r, "offset", 0, 0, 10000); err != nil {
		return f, err
	}
	return f, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	f, err := ParseFilter(r)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	page, err := h.svc.Search(r.Context(), f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, page)
}

func viewer(r *http.Request) Viewer {
	u, _ := auth.CurrentUser(r)
	return Viewer{UserID: u.ID, IsAdmin: u.Role == auth.RoleAdmin}
}

func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	p, err := h.svc.Profile(r.Context(), viewer(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) my(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	h.respond(w, r)(h.svc.My(r.Context(), u.ID))
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	var in Input
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	h.respond(w, r)(h.svc.Save(r.Context(), u.ID, in))
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	h.respond(w, r)(h.svc.Submit(r.Context(), u.ID))
}

func (h *Handler) hide(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	h.respond(w, r)(h.svc.SetHidden(r.Context(), u.ID, true))
}

func (h *Handler) unhide(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	h.respond(w, r)(h.svc.SetHidden(r.Context(), u.ID, false))
}

func (h *Handler) photo(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, MaxPhotoBytes+(1<<20))
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.BadRequest(w, "send the photo as multipart/form-data field \"file\" (max 5 MB)")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxPhotoBytes+1))
	if err != nil {
		httpx.BadRequest(w, "could not read the uploaded file")
		return
	}
	h.respond(w, r)(h.svc.UploadPhoto(r.Context(), u.ID, data))
}

func (h *Handler) adminList(w http.ResponseWriter, r *http.Request) {
	status, err := httpx.QueryEnum(r, "status", []string{visibility.Draft, visibility.Pending, visibility.Live, visibility.Hidden, visibility.Blocked})
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if status == "" {
		status = visibility.Pending
	}
	items, err := h.svc.Queue(r.Context(), status)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) adminGet(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	h.respond(w, r)(h.svc.AdminGet(r.Context(), id))
}

type approveRequest struct {
	Plan   string `json:"plan"`
	EndsOn string `json:"ends_on"`
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var req approveRequest
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			httpx.BadRequest(w, err.Error())
			return
		}
	}
	in := ApproveInput{Plan: req.Plan}
	if req.EndsOn != "" {
		d, err := time.Parse(httpx.DateLayout, req.EndsOn)
		if err != nil {
			httpx.BadRequest(w, "ends_on must be a date in YYYY-MM-DD format")
			return
		}
		in.EndsOn = &d
	}
	h.respond(w, r)(h.svc.Approve(r.Context(), u.ID, id, in))
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var req rejectRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	h.respond(w, r)(h.svc.Reject(r.Context(), id, req.Reason))
}

// respond writes p or maps err.
func (h *Handler) respond(w http.ResponseWriter, r *http.Request) func(Private, error) {
	return func(p Private, err error) {
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, p)
	}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var inc *IncompleteError
	switch {
	case IsValidation(err), errors.Is(err, ErrUnknownSubj):
		httpx.BadRequest(w, err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNoProfile):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	case errors.Is(err, ErrBlocked):
		httpx.Error(w, http.StatusForbidden, httpx.CodeForbidden, err.Error())
	case errors.Is(err, ErrInvalidStatus):
		httpx.Error(w, http.StatusConflict, httpx.CodeInvalidStatus, err.Error())
	case errors.As(err, &inc):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeProfileIncomplete, err.Error())
	case errors.Is(err, ErrBadPhoto):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeInvalidPhoto, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "specialist handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}

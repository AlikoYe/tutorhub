// Package verification is the verified-badge flow: a teacher uploads documents (diploma,
// certificates) that only admins can read, asks for verification, and an admin decides.
package verification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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

const (
	MaxDocumentBytes = 10 << 20
	MaxDocuments     = 5
)

var (
	ErrNotFound    = errors.New("document not found")
	ErrBadDocument = errors.New("document must be a PDF, JPEG or PNG file up to 10 MB")
	ErrTooMany     = errors.New("at most 5 documents")
	ErrNoDocuments = errors.New("upload at least one document first")
	ErrState       = errors.New("verification was already requested or granted")
)

var docExt = map[string]string{"application/pdf": "pdf", "image/jpeg": "jpg", "image/png": "png"}

type Document struct {
	ID           int64     `json:"id"`
	OriginalName string    `json:"original_name"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int       `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`

	fileName string
}

// Request is a pending verification in the admin queue.
type Request struct {
	Specialist specialist.Private `json:"specialist"`
	Documents  []Document         `json:"documents"`
}

// Notifier tells admins and teachers about verification. Implemented by bot.Notifier.
type Notifier interface {
	VerificationRequested(ctx context.Context, adminIDs []int64, specialistName string)
	VerificationDecided(ctx context.Context, teacherID int64, approved bool, note string)
}

type Service struct {
	db       *pgxpool.Pool
	profiles *specialist.Service
	notifier Notifier
	dir      string
	loc      *time.Location
}

func NewService(db *pgxpool.Pool, profiles *specialist.Service, notifier Notifier, uploadDir string, loc *time.Location) *Service {
	return &Service{db: db, profiles: profiles, notifier: notifier, dir: filepath.Join(uploadDir, "documents"), loc: loc}
}

func (s *Service) documents(ctx context.Context, specialistID int64) ([]Document, error) {
	rows, err := s.db.Query(ctx, `SELECT id, original_name, content_type, size_bytes, created_at, file_name
		FROM verification_documents WHERE specialist_id = $1 ORDER BY id`, specialistID)
	if err != nil {
		return nil, fmt.Errorf("select documents: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Document, error) {
		var d Document
		err := r.Scan(&d.ID, &d.OriginalName, &d.ContentType, &d.SizeBytes, &d.CreatedAt, &d.fileName)
		d.CreatedAt = d.CreatedAt.In(s.loc).Truncate(time.Second)
		return d, err
	})
}

// MyDocuments lists the teacher's own documents.
func (s *Service) MyDocuments(ctx context.Context, userID int64) ([]Document, error) {
	id, err := s.profiles.Repo().IDByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.documents(ctx, id)
}

// Upload stores a document for the teacher's profile.
func (s *Service) Upload(ctx context.Context, userID int64, name string, data []byte) (Document, error) {
	id, err := s.profiles.Repo().IDByUser(ctx, userID)
	if err != nil {
		return Document{}, err
	}
	ct := http.DetectContentType(data)
	ext, ok := docExt[ct]
	if !ok || len(data) == 0 || len(data) > MaxDocumentBytes {
		return Document{}, ErrBadDocument
	}
	var n int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM verification_documents WHERE specialist_id = $1`, id).Scan(&n); err != nil {
		return Document{}, err
	}
	if n >= MaxDocuments {
		return Document{}, ErrTooMany
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return Document{}, fmt.Errorf("create document dir: %w", err)
	}
	var rnd [16]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return Document{}, err
	}
	file := hex.EncodeToString(rnd[:]) + "." + ext
	if err := os.WriteFile(filepath.Join(s.dir, file), data, 0o640); err != nil {
		return Document{}, fmt.Errorf("write document: %w", err)
	}
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." || utf8.RuneCountInString(name) > 200 {
		name = "document." + ext
	}
	var docID int64
	err = s.db.QueryRow(ctx, `INSERT INTO verification_documents (specialist_id, file_name, original_name, content_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`, id, file, name, ct, len(data)).Scan(&docID)
	if err != nil {
		_ = os.Remove(filepath.Join(s.dir, file))
		return Document{}, fmt.Errorf("insert document: %w", err)
	}
	docs, err := s.documents(ctx, id)
	if err != nil {
		return Document{}, err
	}
	for _, d := range docs {
		if d.ID == docID {
			return d, nil
		}
	}
	return Document{}, ErrNotFound
}

// Delete removes one of the teacher's documents.
func (s *Service) Delete(ctx context.Context, userID, docID int64) error {
	id, err := s.profiles.Repo().IDByUser(ctx, userID)
	if err != nil {
		return err
	}
	var file string
	err = s.db.QueryRow(ctx, `DELETE FROM verification_documents WHERE id = $1 AND specialist_id = $2 RETURNING file_name`, docID, id).Scan(&file)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	_ = os.Remove(filepath.Join(s.dir, file))
	return nil
}

// Request asks the admins to verify the teacher's documents.
func (s *Service) Request(ctx context.Context, userID int64) (specialist.Private, error) {
	p, err := s.profiles.My(ctx, userID)
	if err != nil {
		return specialist.Private{}, err
	}
	docs, err := s.documents(ctx, p.ID)
	if err != nil {
		return specialist.Private{}, err
	}
	if len(docs) == 0 {
		return specialist.Private{}, ErrNoDocuments
	}
	tag, err := s.db.Exec(ctx, `UPDATE specialists SET verification_status = 'requested', verification_note = '', updated_at = now()
		WHERE id = $1 AND verification_status IN ('none', 'rejected')`, p.ID)
	if err != nil {
		return specialist.Private{}, fmt.Errorf("request verification: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return specialist.Private{}, ErrState
	}
	if admins, err := s.profiles.Repo().AdminIDs(ctx); err == nil {
		s.notifier.VerificationRequested(ctx, admins, p.DisplayName)
	}
	return s.profiles.My(ctx, userID)
}

// Queue lists pending verification requests with their documents.
func (s *Service) Queue(ctx context.Context) ([]Request, error) {
	var ids []int64
	rows, err := s.db.Query(ctx, `SELECT id FROM specialists WHERE verification_status = 'requested' ORDER BY updated_at LIMIT 100`)
	if err != nil {
		return nil, err
	}
	if ids, err = pgx.CollectRows(rows, pgx.RowTo[int64]); err != nil {
		return nil, err
	}
	out := []Request{}
	for _, id := range ids {
		p, err := s.profiles.AdminGet(ctx, id)
		if err != nil {
			return nil, err
		}
		docs, err := s.documents(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, Request{Specialist: p, Documents: docs})
	}
	return out, nil
}

// Decide grants or refuses the verified badge.
func (s *Service) Decide(ctx context.Context, specialistID int64, approve bool, note string) (specialist.Private, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > 500 {
		return specialist.Private{}, &invalidError{"note must be at most 500 characters"}
	}
	status := "rejected"
	if approve {
		status = "verified"
	}
	tag, err := s.db.Exec(ctx, `UPDATE specialists SET verified = $2, verification_status = $3, verification_note = $4, updated_at = now()
		WHERE id = $1`, specialistID, approve, status, note)
	if err != nil {
		return specialist.Private{}, fmt.Errorf("decide verification: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return specialist.Private{}, specialist.ErrNotFound
	}
	p, err := s.profiles.AdminGet(ctx, specialistID)
	if err != nil {
		return specialist.Private{}, err
	}
	s.notifier.VerificationDecided(ctx, p.UserID, approve, note)
	return p, nil
}

// file returns the path and metadata of a document (admins only).
func (s *Service) file(ctx context.Context, docID int64) (string, Document, error) {
	var d Document
	err := s.db.QueryRow(ctx, `SELECT id, original_name, content_type, size_bytes, created_at, file_name
		FROM verification_documents WHERE id = $1`, docID).Scan(&d.ID, &d.OriginalName, &d.ContentType, &d.SizeBytes, &d.CreatedAt, &d.fileName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", d, ErrNotFound
	}
	if err != nil {
		return "", d, err
	}
	return filepath.Join(s.dir, d.fileName), d, nil
}

type invalidError struct{ msg string }

func (e *invalidError) Error() string { return e.msg }

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// MyRoutes mounts inside /api/my/specialist.
func (h *Handler) MyRoutes(r chi.Router) {
	r.Get("/documents", h.myDocuments)
	r.Post("/documents", h.upload)
	r.Delete("/documents/{id}", h.delete)
	r.Post("/verification", h.request)
}

// SpecialistAdminRoutes mounts inside /api/admin/specialists.
func (h *Handler) SpecialistAdminRoutes(r chi.Router) {
	r.Post("/{id}/verification", h.decide)
}

// AdminRoutes mounts under /api/admin.
func (h *Handler) AdminRoutes(r chi.Router) {
	r.Get("/verifications", h.queue)
	r.Get("/documents/{id}/file", h.download)
}

func (h *Handler) myDocuments(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	docs, err := h.svc.MyDocuments(r.Context(), u.ID)
	h.respond(w, r, http.StatusOK, docs, err)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, MaxDocumentBytes+(1<<20))
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.BadRequest(w, "send the document as multipart/form-data field \"file\" (max 10 MB)")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		httpx.BadRequest(w, "could not read the uploaded file")
		return
	}
	d, err := h.svc.Upload(r.Context(), u.ID, header.Filename, data)
	h.respond(w, r, http.StatusCreated, d, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.Delete(r.Context(), u.ID, id); err != nil {
		h.respond(w, r, 0, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) request(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.CurrentUser(r)
	p, err := h.svc.Request(r.Context(), u.ID)
	h.respond(w, r, http.StatusOK, p, err)
}

func (h *Handler) queue(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Queue(r.Context())
	h.respond(w, r, http.StatusOK, items, err)
}

type decideRequest struct {
	Approve *bool  `json:"approve"`
	Note    string `json:"note"`
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	var req decideRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if req.Approve == nil {
		httpx.BadRequest(w, "approve (true or false) is required")
		return
	}
	p, err := h.svc.Decide(r.Context(), id, *req.Approve, req.Note)
	h.respond(w, r, http.StatusOK, p, err)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	path, d, err := h.svc.file(r.Context(), id)
	if err != nil {
		h.respond(w, r, 0, nil, err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		h.respond(w, r, 0, nil, ErrNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", d.ContentType)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", d.OriginalName))
	http.ServeContent(w, r, "", d.CreatedAt, f)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, code int, v any, err error) {
	var inv *invalidError
	switch {
	case err == nil:
		httpx.JSON(w, code, v)
	case errors.As(err, &inv):
		httpx.BadRequest(w, err.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, specialist.ErrNotFound), errors.Is(err, specialist.ErrNoProfile):
		httpx.Error(w, http.StatusNotFound, httpx.CodeNotFound, err.Error())
	case errors.Is(err, ErrBadDocument), errors.Is(err, ErrTooMany):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeInvalidDocument, err.Error())
	case errors.Is(err, ErrNoDocuments):
		httpx.Error(w, http.StatusUnprocessableEntity, httpx.CodeNoDocuments, err.Error())
	case errors.Is(err, ErrState):
		httpx.Error(w, http.StatusConflict, httpx.CodeInvalidStatus, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "verification handler", slog.Any("err", err))
		httpx.Internal(w)
	}
}

package specialist

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
)

// MaxPhotoBytes caps one uploaded photo.
const MaxPhotoBytes = 5 << 20

// PhotoURLPrefix is where photos are served (publicly: <img> tags cannot send auth headers).
const PhotoURLPrefix = "/api/files/photos/"

var photoName = regexp.MustCompile(`^[0-9]+-[0-9a-f]{16}\.(jpg|png|webp)$`)

var photoExt = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}

// PhotoStore keeps specialist photos on disk under <dir>/photos.
type PhotoStore struct {
	dir string
}

func NewPhotoStore(uploadDir string) PhotoStore {
	return PhotoStore{dir: filepath.Join(uploadDir, "photos")}
}

// Save writes data as a new file and returns its public URL. The type is sniffed from the bytes.
func (s PhotoStore) Save(specialistID int64, data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxPhotoBytes {
		return "", ErrBadPhoto
	}
	ext, ok := photoExt[http.DetectContentType(data)]
	if !ok {
		return "", ErrBadPhoto
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", fmt.Errorf("create photo dir: %w", err)
	}
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d-%s.%s", specialistID, hex.EncodeToString(rnd[:]), ext)
	tmp := filepath.Join(s.dir, "."+name+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write photo: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(s.dir, name)); err != nil {
		return "", fmt.Errorf("store photo: %w", err)
	}
	return PhotoURLPrefix + name, nil
}

// Remove deletes the file behind a URL returned by Save; unknown URLs are ignored.
func (s PhotoStore) Remove(url string) {
	name := strings.TrimPrefix(url, PhotoURLPrefix)
	if photoName.MatchString(name) {
		_ = os.Remove(filepath.Join(s.dir, name))
	}
}

// Handler serves GET /api/files/photos/{name}.
func (s PhotoStore) Handler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	path := filepath.Join(s.dir, name)
	if !photoName.MatchString(name) {
		httpx.NotFound(w, r)
		return
	}
	if _, err := os.Stat(path); err != nil {
		httpx.NotFound(w, r)
		return
	}
	// Every upload gets a new name, so a file never changes.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

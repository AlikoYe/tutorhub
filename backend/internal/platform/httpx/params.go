package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// DateLayout is the wire format for calendar dates (YYYY-MM-DD).
const DateLayout = time.DateOnly

// PathID parses a positive int64 chi URL parameter.
func PathID(r *http.Request, name string) (int64, error) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("path parameter %q must be a positive integer", name)
	}
	return id, nil
}

// QueryDate parses a required YYYY-MM-DD query parameter. The result is
// midnight UTC of that date; converting to a venue's local day is the
// service layer's job.
func QueryDate(r *http.Request, name string) (time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return time.Time{}, fmt.Errorf("query parameter %q is required (YYYY-MM-DD)", name)
	}
	d, err := time.Parse(DateLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("query parameter %q must be a date in YYYY-MM-DD format", name)
	}
	return d, nil
}

// QueryInt parses a required integer query parameter within [min, max].
func QueryInt(r *http.Request, name string, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, fmt.Errorf("query parameter %q is required", name)
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("query parameter %q must be an integer between %d and %d", name, min, max)
	}
	return n, nil
}

// QueryIntDefault is QueryInt for an optional parameter.
func QueryIntDefault(r *http.Request, name string, def, min, max int) (int, error) {
	if r.URL.Query().Get(name) == "" {
		return def, nil
	}
	return QueryInt(r, name, min, max)
}

// QueryID parses a required positive int64 query parameter.
func QueryID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("query parameter %q must be a positive integer", name)
	}
	return id, nil
}

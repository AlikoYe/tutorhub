package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/config"
)

func TestHealth(t *testing.T) {
	e := newEnv(t)
	var body map[string]string
	e.mustCall(http.MethodGet, "/api/health", 0, nil, http.StatusOK, &body)
	if body["status"] != "ok" {
		t.Fatalf("body %v", body)
	}
}

func TestMe(t *testing.T) {
	e := newEnv(t)
	e.wantError(e.call(http.MethodGet, "/api/me", 0, nil), http.StatusUnauthorized, "unauthorized")

	var me struct {
		TelegramID int64  `json:"telegram_id"`
		Role       string `json:"role"`
		IsAdmin    bool   `json:"is_admin"`
		Specialist any    `json:"specialist"`
	}
	e.mustCall(http.MethodGet, "/api/me", 11, nil, http.StatusOK, &me)
	if me.TelegramID != 11 || me.Role != "user" || me.IsAdmin {
		t.Fatalf("me = %+v", me)
	}

	e.user(12, true)
	e.mustCall(http.MethodGet, "/api/me", 12, nil, http.StatusOK, &me)
	if me.Role != "admin" || !me.IsAdmin {
		t.Fatalf("admin me = %+v", me)
	}
}

func TestSubjects(t *testing.T) {
	e := newEnv(t)
	var list []struct {
		Slug     string `json:"slug"`
		NameKK   string `json:"name_kk"`
		Category string `json:"category"`
	}
	e.mustCall(http.MethodGet, "/api/subjects", 1, nil, http.StatusOK, &list)
	if len(list) < 40 || list[0].Slug != "math" || list[0].NameKK == "" {
		t.Fatalf("got %d subjects, first %+v", len(list), list[0])
	}
}

// Outside dev only signed initData logs in; the dev bypass is off.
func TestProdAcceptsOnlySignedInitData(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.AppEnv = config.EnvProd })
	e.wantError(e.call(http.MethodGet, "/api/me", 5, nil), http.StatusUnauthorized, "unauthorized")

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "tma "+signInitData(77, time.Now().Add(-time.Minute)))
	if r := e.do(req); r.code != http.StatusOK {
		t.Fatalf("signed initData: %d %s", r.code, r.body)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "tma "+signInitData(77, time.Now().Add(-48*time.Hour)))
	e.wantError(e.do(req), http.StatusUnauthorized, "unauthorized")
}

func TestUnknownRouteIsJSON404(t *testing.T) {
	e := newEnv(t)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nope", nil))
	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

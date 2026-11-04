package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func (e *env) page(path string) (int, string, http.Header) {
	e.t.Helper()
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String(), rec.Header()
}

func mustContain(t *testing.T, body string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(body, p) {
			t.Errorf("page does not contain %q", p)
		}
	}
}

func TestPublicTeacherPage(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(500, e.profileInput("Айгерим <Сапарова>", "ent-math", 6000))
	path := fmt.Sprintf("/t/%d", id)

	code, body, h := e.page(path)
	if code != http.StatusOK || !strings.HasPrefix(h.Get("Content-Type"), "text/html") {
		t.Fatalf("teacher page %d %q", code, h.Get("Content-Type"))
	}
	mustContain(t, body,
		`<title>Айгерим &lt;Сапарова&gt; — ЕНТ: математика | TutorHub</title>`,
		`<link rel="canonical" href="https://tutorhub.example.kz`+path+`">`,
		`hreflang="kk" href="https://tutorhub.example.kz`+path+`?lang=kk"`,
		`<meta property="og:title"`,
		`"@type":"Person"`,
		"6\u00a0000\u00a0₸",
		`https://t.me/tutorhub_test_bot?start=t_`,
		`https://t.me/share/url?`,
	)
	if strings.Contains(body, "Тимирязева") {
		t.Error("public page leaks the exact address")
	}
	if strings.Contains(body, `name="robots" content="noindex"`) {
		t.Error("listed teacher must be indexable")
	}
	// JSON-LD must not be breakable by the name.
	if strings.Contains(body, `"name":"Айгерим <Сапарова>"`) {
		t.Error("JSON-LD is not escaped")
	}

	_, kk, _ := e.page(path + "?lang=kk")
	mustContain(t, kk, `<html lang="kk">`, "ҰБТ: математика", "Telegram арқылы жазылу")
	_, en, _ := e.page(path + "?lang=en")
	mustContain(t, en, "UNT: mathematics", "Book in Telegram")

	// The view is counted (anonymous).
	var views int
	if err := e.db.QueryRow(t.Context(), `SELECT count(*) FROM profile_events WHERE specialist_id = $1 AND kind = 'view' AND user_id IS NULL`, id).Scan(&views); err != nil || views != 3 {
		t.Fatalf("views %d %v", views, err)
	}

	// Expired: still reachable by link but noindex. Hidden: 404.
	e.exec(`UPDATE subscriptions SET starts_at = now() - interval '40 days', ends_at = now() - interval '1 day' WHERE specialist_id = $1`, id)
	_, body, _ = e.page(path)
	mustContain(t, body, `name="robots" content="noindex"`)
	e.mustCall(http.MethodPost, "/api/my/specialist/hide", 500, nil, http.StatusOK, nil)
	if code, _, _ := e.page(path); code != http.StatusNotFound {
		t.Fatalf("hidden teacher page %d", code)
	}
	if code, _, _ := e.page("/t/abc"); code != http.StatusNotFound {
		t.Fatalf("bad id %d", code)
	}
}

func TestPublicCatalogueAndSitemap(t *testing.T) {
	e := newEnv(t)
	math := e.liveSpecialist(510, e.profileInput("Math One", "ent-math", 6000))
	e.liveSpecialist(511, e.profileInput("English One", "ielts", 9000))
	e.user(512, false)
	draft := e.id(`INSERT INTO specialists (user_id, display_name) VALUES (512, 'Draft') RETURNING id`)
	_ = draft

	code, body, _ := e.page("/c/almaty/ent-math")
	if code != http.StatusOK {
		t.Fatalf("subject page %d", code)
	}
	mustContain(t, body, "<title>Репетиторы: ЕНТ: математика — Алматы | TutorHub</title>", "Math One", fmt.Sprintf("/t/%d", math), "IELTS")
	if strings.Contains(body, "English One") {
		t.Error("subject page lists a teacher of another subject")
	}

	code, body, _ = e.page("/c/almaty/chess")
	if code != http.StatusOK || !strings.Contains(body, `content="noindex"`) {
		t.Fatalf("empty subject page %d, want 200 + noindex", code)
	}
	code, body, _ = e.page("/c/almaty?lang=en")
	if code != http.StatusOK {
		t.Fatalf("city page %d", code)
	}
	mustContain(t, body, "Tutors in Almaty", "Math One", "English One")
	for _, p := range []string{"/c/astana", "/c/almaty/no-such-subject"} {
		if code, _, _ := e.page(p); code != http.StatusNotFound {
			t.Errorf("%s: %d", p, code)
		}
	}

	code, body, h := e.page("/sitemap.xml")
	if code != http.StatusOK || !strings.HasPrefix(h.Get("Content-Type"), "application/xml") {
		t.Fatalf("sitemap %d", code)
	}
	mustContain(t, body, "<loc>https://tutorhub.example.kz/c/almaty</loc>", "<loc>https://tutorhub.example.kz/c/almaty/ent-math</loc>",
		"<loc>https://tutorhub.example.kz/c/almaty/ielts?lang=kk</loc>", fmt.Sprintf("<loc>https://tutorhub.example.kz/t/%d</loc>", math))
	if strings.Contains(body, "/c/almaty/chess") || strings.Contains(body, fmt.Sprintf("/t/%d<", draft)) {
		t.Error("sitemap lists empty or private pages")
	}

	_, body, _ = e.page("/robots.txt")
	mustContain(t, body, "Disallow: /api/", "Sitemap: https://tutorhub.example.kz/sitemap.xml")
}

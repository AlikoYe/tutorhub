package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
)

// echoUser responds with the user found in the request context.
var echoUser = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r)
	if !ok {
		http.Error(w, "no user in context", http.StatusInternalServerError)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
})

// fakeUsers records upserts and returns a fixed role or error.
type fakeUsers struct {
	role    string
	blocked bool
	err     error
	saved   []User
}

func (f *fakeUsers) Upsert(_ context.Context, u User) (string, bool, error) {
	f.saved = append(f.saved, u)
	if f.err != nil {
		return "", false, f.err
	}
	if f.role == "" {
		return RoleUser, f.blocked, nil
	}
	return f.role, f.blocked, nil
}

func serve(t *testing.T, opts Options, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	if opts.Users == nil {
		opts.Users = &fakeUsers{}
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return testNow }
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	Middleware(opts)(echoUser).ServeHTTP(rec, req)
	return rec
}

func TestMiddleware_ValidTMA(t *testing.T) {
	initData := signInitData(t, validFields(testNow.Add(-time.Minute)), testBotToken)

	for _, env := range []string{"prod", "dev"} {
		rec := serve(t, Options{BotToken: testBotToken, AppEnv: env}, "tma "+initData)
		if rec.Code != http.StatusOK {
			t.Fatalf("env=%s: status = %d, body = %s", env, rec.Code, rec.Body)
		}
		var u User
		if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}
		if u.ID != 42 {
			t.Errorf("env=%s: user id = %d, want 42", env, u.ID)
		}
	}
}

func TestMiddleware_Rejects(t *testing.T) {
	valid := signInitData(t, validFields(testNow.Add(-time.Minute)), testBotToken)
	expired := signInitData(t, validFields(testNow.Add(-48*time.Hour)), testBotToken)

	tests := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"scheme only", "tma"},
		{"empty credentials", "tma "},
		{"unknown scheme", "Bearer " + valid},
		{"bad signature", "tma " + valid + "0"},
		{"expired", "tma " + expired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, Options{BotToken: testBotToken, AppEnv: "prod"}, tt.header)
			assertUnauthorized(t, rec)
		})
	}
}

func TestMiddleware_DevBypass(t *testing.T) {
	rec := serve(t, Options{AppEnv: "dev"}, "dev 1001")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var u User
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != 1001 {
		t.Errorf("user id = %d, want 1001", u.ID)
	}

	for _, bad := range []string{"dev abc", "dev -5", "dev 0"} {
		assertUnauthorized(t, serve(t, Options{AppEnv: "dev"}, bad))
	}
}

func TestMiddleware_UpsertsUserAndSetsRole(t *testing.T) {
	initData := signInitData(t, validFields(testNow.Add(-time.Minute)), testBotToken)
	users := &fakeUsers{role: "admin"}

	var got User
	h := Middleware(Options{BotToken: testBotToken, AppEnv: "prod", Users: users, Now: func() time.Time { return testNow }})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got, _ = CurrentUser(r) }))
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "tma "+initData)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if len(users.saved) != 1 || users.saved[0].ID != 42 || users.saved[0].DisplayName() != "Ada Lovelace" {
		t.Fatalf("upserted %+v, want user 42 Ada Lovelace", users.saved)
	}
	if got.Role != "admin" {
		t.Errorf("role in context = %q, want admin", got.Role)
	}
}

func TestMiddleware_UpsertFailureIs500(t *testing.T) {
	rec := serve(t, Options{AppEnv: "dev", Users: &fakeUsers{err: errors.New("db down")}}, "dev 7")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestMiddleware_RejectedRequestsDoNotUpsert(t *testing.T) {
	users := &fakeUsers{}
	assertUnauthorized(t, serve(t, Options{AppEnv: "prod", Users: users}, "dev 7"))
	if len(users.saved) != 0 {
		t.Fatalf("upserted %d users for a rejected request", len(users.saved))
	}
}

func TestMiddleware_DevBypassDisabledOutsideDev(t *testing.T) {
	for _, env := range []string{"prod", "staging", "", "Dev", "DEV", "dev ", "development"} {
		t.Run("env="+env, func(t *testing.T) {
			rec := serve(t, Options{BotToken: testBotToken, AppEnv: env}, "dev 1001")
			assertUnauthorized(t, rec)
		})
	}
}

func assertUnauthorized(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body)
	}
	var body httpx.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not an error envelope: %v", err)
	}
	if body.Error != httpx.CodeUnauthorized || body.Message == "" {
		t.Errorf("unexpected error body: %+v", body)
	}
}

func TestRequireAdmin(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for role, want := range map[string]int{RoleAdmin: http.StatusNoContent, RoleUser: http.StatusForbidden} {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/x", nil)
		req = req.WithContext(WithUser(req.Context(), User{ID: 1, Role: role}))
		rec := httptest.NewRecorder()
		RequireAdmin(ok).ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("role %s: status %d, want %d", role, rec.Code, want)
		}
	}
	rec := httptest.NewRecorder()
	RequireAdmin(ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no user: status %d, want 401", rec.Code)
	}
}

func TestMiddleware_BlockedUserIs403(t *testing.T) {
	rec := serve(t, Options{AppEnv: "dev", Users: &fakeUsers{blocked: true}}, "dev 7")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

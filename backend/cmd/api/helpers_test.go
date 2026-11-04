package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"github.com/AlikoYe/tutorhub/backend/internal/bot"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/config"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/dbtest"
)

// contract validates every response in these tests against docs/api.yaml.
var (
	contractOnce sync.Once
	contract     validator.Validator
	contractErr  error
	contractMu   sync.Mutex
)

func loadContract(t *testing.T) validator.Validator {
	t.Helper()
	contractOnce.Do(func() {
		spec, err := os.ReadFile("../../../docs/api.yaml")
		if err != nil {
			contractErr = err
			return
		}
		doc, err := libopenapi.NewDocument(spec)
		if err != nil {
			contractErr = err
			return
		}
		v, errs := validator.NewValidator(doc)
		if len(errs) > 0 {
			contractErr = errors.Join(errs...)
			return
		}
		contract = v
	})
	if contractErr != nil {
		t.Fatalf("load docs/api.yaml: %v", contractErr)
	}
	return contract
}

const testBotToken = "123456:TEST-token"

type env struct {
	t        *testing.T
	db       *pgxpool.Pool
	app      *app
	h        http.Handler
	sender   *recordingSender
	notifier *bot.Notifier
	bot      *bot.Bot
	loc      *time.Location
}

type configT = config.Config

type envOption func(*config.Config)

// newEnv migrates a clean database and builds the app in dev mode (dev auth on).
func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	db := dbtest.New(t)
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AppEnv:           config.EnvDev,
		BotToken:         testBotToken,
		Timezone:         loc,
		UploadDir:        t.TempDir(),
		HoldMinutes:      360,
		TrialDays:        30,
		WebAppURL:        "https://app.example.com",
		BotUsername:      "tutorhub_test_bot",
		SiteURL:          "https://tutorhub.example.kz",
		ReminderBefore:   2 * time.Hour,
		ReminderInterval: time.Minute,
	}
	for _, o := range opts {
		o(&cfg)
	}
	log := slog.New(slog.DiscardHandler)
	sender := &recordingSender{}
	links := bot.Links{WebAppURL: cfg.WebAppURL, BotUsername: cfg.BotUsername}
	notifier := bot.NewNotifier(sender, links, loc, log)
	a := newApp(cfg, log, db, notifier)
	tg := bot.NewWithSender(sender, links, log)
	a.registerBot(tg)
	loadContract(t)
	e := &env{t: t, db: db, app: a, h: a.router(), sender: sender, notifier: notifier, bot: tg, loc: loc}
	t.Cleanup(notifier.Wait)
	return e
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("exec %q: %v", sql, err)
	}
}

func (e *env) id(sql string, args ...any) int64 {
	e.t.Helper()
	var id int64
	if err := e.db.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		e.t.Fatalf("query %q: %v", sql, err)
	}
	return id
}

// user creates a user row (role user unless admin).
func (e *env) user(id int64, admin bool) {
	e.t.Helper()
	role := "user"
	if admin {
		role = "admin"
	}
	e.exec(`INSERT INTO users (telegram_id, name, role) VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO UPDATE SET role = EXCLUDED.role`, id, fmt.Sprintf("User %d", id), role)
}

type response struct {
	code   int
	body   []byte
	header http.Header
}

func (r response) errorCode() string {
	var e struct{ Error string }
	_ = json.Unmarshal(r.body, &e)
	return e.Error
}

// call sends a JSON request as dev user `user` (0 = no auth) and checks the response against the contract.
func (e *env) call(method, path string, user int64, body any) response {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != 0 {
		req.Header.Set("Authorization", fmt.Sprintf("dev %d", user))
	}
	return e.do(req)
}

// upload sends a multipart file as field "file".
func (e *env) upload(path string, user int64, filename string, content []byte) response {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		e.t.Fatal(err)
	}
	fw.Write(content)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if user != 0 {
		req.Header.Set("Authorization", fmt.Sprintf("dev %d", user))
	}
	return e.do(req)
}

// do serves req and validates the response against docs/api.yaml.
func (e *env) do(req *http.Request) response {
	e.t.Helper()
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)

	res := rec.Result()
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "image/") && !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/pdf") {
		contractMu.Lock()
		ok, errs := contract.ValidateHttpResponse(req, res)
		contractMu.Unlock()
		if !ok {
			var msgs []string
			for _, ve := range errs {
				msgs = append(msgs, ve.Error())
				for _, se := range ve.SchemaValidationErrors {
					msgs = append(msgs, "  "+se.FieldPath+": "+se.Reason)
				}
			}
			e.t.Errorf("%s %s -> %d does not match docs/api.yaml:\n%s\nbody: %s", req.Method, req.URL, rec.Code, strings.Join(msgs, "\n"), rec.Body)
		}
	}
	return response{code: rec.Code, body: rec.Body.Bytes(), header: rec.Header()}
}

func (e *env) mustCall(method, path string, user int64, body any, wantCode int, out any) response {
	e.t.Helper()
	r := e.call(method, path, user, body)
	if r.code != wantCode {
		e.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, r.code, wantCode, r.body)
	}
	if out != nil {
		if err := json.Unmarshal(r.body, out); err != nil {
			e.t.Fatalf("decode %s: %v", r.body, err)
		}
	}
	return r
}

func (e *env) wantError(r response, code int, errCode string) {
	e.t.Helper()
	if r.code != code || r.errorCode() != errCode {
		e.t.Fatalf("got %d %s, want %d %s", r.code, r.body, code, errCode)
	}
}

// signInitData builds a valid Telegram initData string for user id, signed with testBotToken.
func signInitData(id int64, authDate time.Time) string {
	fields := map[string]string{
		"auth_date": strconv.FormatInt(authDate.Unix(), 10),
		"user":      fmt.Sprintf(`{"id":%d,"first_name":"Signed","username":"signed_user","language_code":"ru"}`, id),
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + fields[k]
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(testBotToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

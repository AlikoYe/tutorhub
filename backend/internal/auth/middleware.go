package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
)

// UserStore saves the authenticated user and returns their role and whether they are blocked.
type UserStore interface {
	Upsert(ctx context.Context, u User) (role string, blocked bool, err error)
}

// Options configures Middleware.
type Options struct {
	BotToken string
	// Users is required: every authenticated request upserts the user so
	// foreign keys to users always resolve.
	Users UserStore
	// AppEnv is the raw APP_ENV value. The dev bypass is enabled if and only
	// if it equals "dev" exactly; callers cannot enable it any other way.
	AppEnv string
	Logger *slog.Logger
	// Now is overridable for tests. Defaults to time.Now.
	Now func() time.Time
}

// Middleware authenticates requests using the Authorization header:
//
//	Authorization: tma <initData>       (always)
//	Authorization: dev <telegram_id>    (only when APP_ENV=dev)
//
// On success the user is upserted and stored in the request context;
// otherwise it responds 401 with the standard error body.
func Middleware(opts Options) func(http.Handler) http.Handler {
	if opts.Users == nil {
		panic("auth: Options.Users is required")
	}
	devBypass := opts.AppEnv == "dev"
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme, credentials, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			if !ok || credentials == "" {
				unauthorized(w, "missing or malformed Authorization header")
				return
			}

			var user User
			switch {
			case strings.EqualFold(scheme, "tma"):
				data, err := Validate(credentials, opts.BotToken, now())
				if err != nil {
					log.DebugContext(r.Context(), "initData rejected", slog.String("reason", err.Error()))
					if errors.Is(err, ErrNoBotToken) {
						unauthorized(w, "telegram auth is not configured on this server")
						return
					}
					unauthorized(w, err.Error())
					return
				}
				user = data.User

			case devBypass && strings.EqualFold(scheme, "dev"):
				id, err := strconv.ParseInt(credentials, 10, 64)
				if err != nil || id <= 0 {
					unauthorized(w, "dev auth expects a positive telegram id")
					return
				}
				user = User{ID: id, FirstName: "Dev", LastName: strconv.FormatInt(id, 10), Username: "dev_" + credentials}

			default:
				unauthorized(w, "unsupported Authorization scheme")
				return
			}

			role, blocked, err := opts.Users.Upsert(r.Context(), user)
			if err != nil {
				log.ErrorContext(r.Context(), "upsert user", slog.Int64("telegram_id", user.ID), slog.Any("err", err))
				httpx.Internal(w)
				return
			}
			if blocked {
				httpx.Error(w, http.StatusForbidden, httpx.CodeForbidden, "your account is blocked")
				return
			}
			user.Role = role

			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}

func unauthorized(w http.ResponseWriter, msg string) {
	httpx.Error(w, http.StatusUnauthorized, httpx.CodeUnauthorized, msg)
}

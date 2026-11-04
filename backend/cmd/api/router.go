package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/auth"
	"github.com/AlikoYe/tutorhub/backend/internal/bot"
	"github.com/AlikoYe/tutorhub/backend/internal/notify"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/config"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
	"github.com/AlikoYe/tutorhub/backend/internal/subject"
)

// app wires every domain service; shared by the HTTP router, the bot and the worker.
type app struct {
	cfg      config.Config
	log      *slog.Logger
	db       *pgxpool.Pool
	notifier *bot.Notifier

	subjects *subject.Repo
}

func newApp(cfg config.Config, log *slog.Logger, db *pgxpool.Pool, notifier *bot.Notifier) *app {
	return &app{
		cfg:      cfg,
		log:      log,
		db:       db,
		notifier: notifier,
		subjects: subject.NewRepo(db),
	}
}

// registerBot adds the domain commands and callbacks to the bot.
func (a *app) registerBot(_ *bot.Bot) {}

// worker returns the background worker with every domain job.
func (a *app) worker() *notify.Worker {
	return notify.NewWorker(a.cfg.ReminderInterval, a.log)
}

func (a *app) router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.RequestLogger(a.log))
	r.Use(httpx.Recoverer(a.log))
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler(a.db))

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(auth.Options{
				BotToken: a.cfg.BotToken,
				AppEnv:   a.cfg.AppEnv,
				Users:    auth.NewUserRepo(a.db),
				Logger:   a.log,
			}))
			r.Get("/me", auth.MeHandler(nil, a.log))
			r.Get("/subjects", subject.ListHandler(a.subjects, a.log))
		})
	})
	return r
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, httpx.CodeUnavailable, "database is unreachable")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

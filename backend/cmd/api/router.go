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
	"github.com/AlikoYe/tutorhub/backend/internal/availability"
	"github.com/AlikoYe/tutorhub/backend/internal/bot"
	"github.com/AlikoYe/tutorhub/backend/internal/geo"
	"github.com/AlikoYe/tutorhub/backend/internal/lesson"
	"github.com/AlikoYe/tutorhub/backend/internal/notify"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/config"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
	"github.com/AlikoYe/tutorhub/backend/internal/subject"
)

// app wires every domain service; shared by the HTTP router, the bot and the worker.
type app struct {
	cfg      config.Config
	log      *slog.Logger
	db       *pgxpool.Pool
	notifier *bot.Notifier

	subjects    *subject.Repo
	photos      specialist.PhotoStore
	specialists *specialist.Service
	schedules   *availability.Service
	lessons     *lesson.Service
}

func newApp(cfg config.Config, log *slog.Logger, db *pgxpool.Pool, notifier *bot.Notifier) *app {
	a := &app{
		cfg:      cfg,
		log:      log,
		db:       db,
		notifier: notifier,
		subjects: subject.NewRepo(db),
		photos:   specialist.NewPhotoStore(cfg.UploadDir),
	}
	links := bot.Links{WebAppURL: cfg.WebAppURL, BotUsername: cfg.BotUsername}
	a.specialists = specialist.NewService(specialist.NewRepo(db, cfg.Timezone.String()), specialist.Options{
		Photos:       a.photos,
		Notifier:     notifier,
		PersonalLink: links.SpecialistLink,
		TrialDays:    cfg.TrialDays,
		Location:     cfg.Timezone,
		Log:          log,
	})
	a.schedules = availability.NewService(availability.NewRepo(db), a.specialists, cfg.Timezone)
	a.lessons = lesson.NewService(lesson.NewRepo(db, cfg.Timezone), a.schedules, a.specialists.Repo(), notifier, cfg.HoldMinutes, cfg.Timezone, log)
	a.specialists.SetAddressAccess(a.lessons)
	return a
}

// registerBot adds the domain commands and callbacks to the bot.
func (a *app) registerBot(b *bot.Bot) {
	b.RegisterSpecialists(a.specialists)
	b.RegisterLessons(a.lessons, a.cfg.Timezone)
}

// worker returns the background worker with every domain job.
func (a *app) worker() *notify.Worker {
	return notify.NewWorker(a.cfg.ReminderInterval, a.log, a.lessons.Jobs(a.cfg.ReminderBefore)...)
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
		r.Get("/files/photos/{name}", a.photos.Handler)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(auth.Options{
				BotToken: a.cfg.BotToken,
				AppEnv:   a.cfg.AppEnv,
				Users:    auth.NewUserRepo(a.db),
				Logger:   a.log,
			}))
			r.Get("/me", auth.MeHandler(a.specialists, a.log))
			r.Get("/subjects", subject.ListHandler(a.subjects, a.log))
			r.Get("/meta", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, http.StatusOK, geo.CurrentMeta()) })

			specialists := specialist.NewHandler(a.specialists, a.log)
			schedules := availability.NewHandler(a.schedules, a.log)
			lessons := lesson.NewHandler(a.lessons, a.log)
			r.Route("/specialists", func(r chi.Router) {
				specialists.CatalogueRoutes(r)
				r.Get("/{id}/slots", schedules.Slots)
			})
			r.Route("/my/specialist", specialists.MyRoutes)
			r.Route("/lessons", lessons.StudentRoutes)
			r.Route("/teacher", func(r chi.Router) {
				lessons.TeacherRoutes(r)
				r.Route("/availability", schedules.TeacherRoutes)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Route("/specialists", specialists.AdminRoutes)
			})
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

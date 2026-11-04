// Command api runs the TutorHub HTTP API, the Telegram bot and the background worker in one
// process, with graceful shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // the zone database must not depend on the container image

	"golang.org/x/sync/errgroup"

	"github.com/AlikoYe/tutorhub/backend/internal/auth"
	"github.com/AlikoYe/tutorhub/backend/internal/bot"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/config"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/database"
	"github.com/AlikoYe/tutorhub/backend/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logging.New(cfg.LogLevel, cfg.IsDev())
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := auth.NewUserRepo(pool).EnsureAdmins(ctx, cfg.AdminIDs); err != nil {
		return err
	}

	links := bot.Links{WebAppURL: cfg.WebAppURL, BotUsername: cfg.BotUsername}
	// The bot is optional in dev: without a token, messages are only logged.
	var tg *bot.Bot
	if cfg.BotToken != "" {
		if tg, err = bot.New(cfg.BotToken, links, log); err != nil {
			return err
		}
	} else {
		log.Warn("BOT_TOKEN is empty: telegram bot disabled, messages are logged and only dev auth works")
		tg = bot.NewWithSender(bot.LogSender{Log: log}, links, log)
	}
	notifier := bot.NewNotifier(tg.Sender(), links, cfg.Timezone, log)
	app := newApp(cfg, log, pool, notifier)
	app.registerBot(tg)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Info("http server listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})
	if cfg.BotToken != "" {
		g.Go(func() error { return tg.Run(gctx) })
	}
	g.Go(func() error { return app.worker().Run(gctx) })

	err = g.Wait()
	// Let in-flight Telegram sends finish; each is bounded by bot.SendTimeout.
	notifier.Wait()
	if err != nil {
		return err
	}
	log.Info("stopped cleanly")
	return nil
}

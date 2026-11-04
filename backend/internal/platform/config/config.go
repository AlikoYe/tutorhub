// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Supported values of APP_ENV.
const (
	EnvDev     = "dev"
	EnvStaging = "staging"
	EnvProd    = "prod"
)

type Config struct {
	// AppEnv is one of dev, staging, prod. Only "dev" enables dev-only
	// conveniences such as the "Authorization: dev <id>" bypass.
	AppEnv   string
	HTTPAddr string
	LogLevel string

	DatabaseURL string

	// BotToken is required outside dev: it is both the bot credential and
	// the key used to validate Telegram initData.
	BotToken string
	// WebAppURL is the public HTTPS URL of the Mini App, used for the
	// "Open" buttons the bot sends. Optional.
	WebAppURL string
	// BotUsername (without @) builds personal links t.me/<bot>?start=t_<id>. Optional.
	BotUsername string
	// AdminIDs are Telegram ids promoted to role admin at startup.
	AdminIDs []int64

	// Timezone is the zone lessons, schedules and stats days are expressed in.
	Timezone *time.Location
	// UploadDir is where teacher photos and documents are stored.
	UploadDir string

	ReminderInterval time.Duration
	ShutdownTimeout  time.Duration

	// HoldMinutes is how long a lesson request waits for the teacher before it expires.
	HoldMinutes int
	// ReminderBefore is how long before the start a confirmed lesson is reminded.
	ReminderBefore time.Duration
}

func (c Config) IsDev() bool { return c.AppEnv == EnvDev }

// Load reads configuration from the environment and validates it.
func Load() (Config, error) {
	var errs []error

	cfg := Config{
		AppEnv:      strings.TrimSpace(getenv("APP_ENV", EnvProd)),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BotToken:    os.Getenv("BOT_TOKEN"),
		WebAppURL:   os.Getenv("WEBAPP_URL"),
		BotUsername: strings.TrimPrefix(strings.TrimSpace(os.Getenv("BOT_USERNAME")), "@"),
		UploadDir:   getenv("UPLOAD_DIR", "./data/uploads"),
	}

	switch cfg.AppEnv {
	case EnvDev, EnvStaging, EnvProd:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be one of dev, staging, prod; got %q", cfg.AppEnv))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if cfg.BotToken == "" && !cfg.IsDev() {
		errs = append(errs, errors.New("BOT_TOKEN is required when APP_ENV is not dev"))
	}
	if cfg.WebAppURL != "" && !strings.HasPrefix(cfg.WebAppURL, "https://") {
		errs = append(errs, errors.New("WEBAPP_URL must start with https:// (Telegram requirement)"))
	}

	var err error
	if cfg.AdminIDs, err = idListEnv("ADMIN_TELEGRAM_IDS"); err != nil {
		errs = append(errs, err)
	}
	tz := getenv("TIMEZONE", "Asia/Almaty")
	if cfg.Timezone, err = time.LoadLocation(tz); err != nil {
		errs = append(errs, fmt.Errorf("TIMEZONE must be an IANA zone like Asia/Almaty; got %q", tz))
	}
	if cfg.ReminderInterval, err = durationEnv("REMINDER_INTERVAL", time.Minute); err != nil {
		errs = append(errs, err)
	}
	if cfg.ShutdownTimeout, err = durationEnv("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		errs = append(errs, err)
	}
	if cfg.ReminderBefore, err = durationEnv("REMINDER_BEFORE", 2*time.Hour); err != nil {
		errs = append(errs, err)
	}
	if cfg.HoldMinutes, err = positiveIntEnv("HOLD_MINUTES", 360); err != nil {
		errs = append(errs, err)
	}

	return cfg, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration like 30s or 1m; got %q", key, v)
	}
	return d, nil
}

func idListEnv(key string) ([]int64, error) {
	var ids []int64
	for _, part := range strings.Split(os.Getenv(key), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%s must be a comma-separated list of telegram ids; got %q", key, part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func positiveIntEnv(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer; got %q", key, v)
	}
	return n, nil
}

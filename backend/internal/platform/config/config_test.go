package config

import "testing"

func TestLoadDefaultsAndValidation(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("BOT_TOKEN", "")
	t.Setenv("ADMIN_TELEGRAM_IDS", " 1, 22 ,")
	t.Setenv("BOT_USERNAME", "@tutor_bot")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AdminIDs) != 2 || cfg.AdminIDs[1] != 22 || cfg.BotUsername != "tutor_bot" ||
		cfg.Timezone.String() != "Asia/Almaty" || cfg.HoldMinutes != 360 {
		t.Fatalf("cfg = %+v", cfg)
	}

	t.Setenv("APP_ENV", "prod")
	t.Setenv("ADMIN_TELEGRAM_IDS", "abc")
	t.Setenv("TIMEZONE", "Mars/Olympus")
	if _, err := Load(); err == nil {
		t.Fatal("want errors for missing BOT_TOKEN, bad admin ids and bad timezone")
	}
}

# Decisions

Choices made while building unattended. One line each: what and why.

- Branches: used the stacked milestone branches the owner asked for (m0-foundation, m1-catalogue, ...) instead of the session's default branch; the final branch is also pushed to `claude/dazzling-mendel-36zi2o`.
- PRs are opened through the GitHub API (MCP tools) because the `gh` CLI in this sandbox has no valid token.
- Go 1.26.4 (same as TableBot), downloaded with GOTOOLCHAIN; PostgreSQL 16 runs locally from the apt binaries (initdb in /var/lib/pgdata) because Docker cannot run here.
- Module path `github.com/AlikoYe/tutorhub/backend`.
- One app time zone (`TIMEZONE`, default Asia/Almaty) instead of per-city zones: Kazakhstan uses a single zone (UTC+5) since 2024 and the MVP launches in Almaty only.
- Admins are bootstrapped from `ADMIN_TELEGRAM_IDS` at startup (users.role = admin); there is no UI to grant admin.
- All `/api` endpoints except `/health` (and later public photo files) require Telegram auth, including the catalogue; the public website (M5) is server-rendered and does not use the JSON API.
- UI and bot texts are Russian first; Kazakh/English UI is M5.
- Frontend uses vue-router web history (not hash history) because Telegram puts launch params in the URL hash.
- Frontend has no Pinia: one small reactive store is enough for now.
- `HOLD_MINUTES` default is 360 (6 h) instead of TableBot's 15 min: teachers answer slower than restaurants; a hold never outlives the lesson start.
- Subject taxonomy: 47 subjects in 8 categories (school, exam, language, it, music, arts, consulting, other) with ru/kk/en names.

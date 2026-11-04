# TutorHub

Telegram Mini App: a catalogue of teachers, tutors and consultants in Kazakhstan (Almaty first).
Students search and request lessons; teachers pay a monthly subscription to be visible.
Read `PLAN.md` for the product and the milestones; `DECISIONS.md` for choices made on the way.

Stack: Go (chi, pgx/v5, goose, go-telegram/bot, slog), PostgreSQL 16, Vue 3 + Vite,
Docker Compose + Caddy. Patterns adapted from TableBot (github.com/AlikoYe/TableBot).

## Layout

```
backend/
  cmd/api/              main.go (HTTP + bot + worker), router.go (app wiring + all routes)
  cmd/migrate/          goose runner (up | down | status)
  internal/
    auth/               initData validation, middleware (upserts user), RequireAdmin, /me, UserRepo
    subject/            the seeded subject taxonomy
    bot/                Telegram: Sender interface, Notifier, Links, command/callback dispatch;
                        every user-facing text is in bot/texts*.go
    notify/             background worker: runs registered jobs every REMINDER_INTERVAL
    platform/           config, database, dbtest, httpx (JSON, errors, params), logging
  migrations/           000N_*.sql (+ schema_test.go)
  seed/dev.sql          dev data
docs/api.yaml           the API contract (OpenAPI 3.1)
frontend/               Vue 3 + Vite Mini App
deploy/                 compose (base + prod override), web.Dockerfile, Caddyfile, scripts
```

Each domain package uses **handler -> service -> repo**:
- `handler.go`: parse/validate HTTP input, call the service, map errors to JSON.
- `service.go`: business rules, domain errors (`ErrNotFound`, ...), no HTTP.
- `repo.go`: SQL via pgx; returns domain types and domain errors.
A package without business rules (e.g. `subject`) may skip the service.

## Commands (run from the repo root)

- `make dev` — Postgres in Docker, migrations, API on :8080 and Vite on :5173.
- `make seed` — demo teachers in Almaty; `ADMIN=<telegram_id>` becomes admin (default 1).
- `make test` — unit tests; DB tests are skipped (it unsets `TEST_DATABASE_URL`).
- `make test-db` — all tests incl. DB integration tests against `tutorhub_test`. Each DB test
  wipes and re-migrates the `public` schema (`dbtest.New`); it refuses a database whose name
  lacks "test". By hand (any Postgres 16):
  `TEST_DATABASE_URL=postgres://tutorhub:tutorhub@localhost:5432/tutorhub_test?sslmode=disable go test -race -count=1 -p 1 ./...`
  in `backend/`. `-p 1` is required: DB packages share one database.
- `make lint` — gofmt + go vet. `make lint-api` — redocly lint of `docs/api.yaml`.
- HTTP tests in `cmd/api/*_test.go` validate every response against `docs/api.yaml`
  (libopenapi-validator); an undocumented status code or field fails the test.
- Frontend: `cd frontend && npm run build`.

## Conventions

- JSON errors are always `{"error": "<code>", "message": "<text>"}` (use `httpx.Error`;
  codes live in `httpx/json.go` and must match `docs/api.yaml`).
- Times in the API are RFC 3339 with offset in the app zone (`TIMEZONE`, default Asia/Almaty).
  Dates are `YYYY-MM-DD` in that zone.
- `docs/api.yaml` is the contract. Change it first; call out every change in the PR.
- Code comments: short, one line, only where the code is not obvious.
- Code must be gofmt + go vet clean.
- Auth: `Authorization: tma <initData>` always; `dev <telegram_id>` only when `APP_ENV=dev`.
  Admin endpoints sit behind `auth.RequireAdmin` (role from `users.role`;
  `ADMIN_TELEGRAM_IDS` are promoted at startup).
- Telegram: talk to it only through `bot.Sender` (fake it in tests). With an empty
  `BOT_TOKEN`, `bot.LogSender` logs every message instead, so dev runs the same code.
  Domain notifications go through `bot.Notifier` (background, detached context, 10 s
  timeout, failures logged at warn and never fail a request). Tests call `Notifier.Wait()`.
- UI and bot texts are Russian for now (kk/en in M5).
- Prices are configuration, not code.

## Workflow

- Stacked milestone branches (m0-foundation, m1-catalogue, ...), each PR into the previous one.
- Run `make lint`, `make test-db` and `npm run build` before pushing.
- PR description: summary, API contract changes, "How I tested" with real commands and results.
  Never merge.
- Never commit `.env`, bot tokens or other secrets.

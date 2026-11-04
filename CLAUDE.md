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
    geo/                cities/districts (Almaty) and fixed vocabularies (levels, formats, languages); /meta
    visibility/         THE listing rule + ranking SQL (live AND running subscription); use it, never re-type it
    specialist/         profiles: owner editor, submit, admin verification, photos, catalogue search
    availability/       weekly template + days off, free-slot computation (pure, unit-tested)
    lessonstatus/       THE blocking / effective-status SQL for lessons; use it, never re-type it
    lesson/             lesson requests, confirm/decline (HTTP + bot), cancel, students, worker jobs
    subscription/       plans/prices, admin activation/extension, founding period, expiry worker
    stats/              profile_events recorder, weekly teacher stats, admin dashboard
    review/             reviews after a completed lesson (one per student per teacher), moderation
    moderation/         profile reports, blocking users
    verification/       verified badge: documents (admins only, UPLOAD_DIR/documents) + decision
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
- Specialist status: draft -> pending (submit) -> live (admin approve, starts trial/founding plan) or
  back to draft with rejection_reason; live <-> hidden by the teacher; blocked by admins.
  Listed in search iff `visibility.ListedSQL`; the profile page opens while `status = live`
  (link keeps working after the subscription ends); owner and admins always see it.
- Photos live in `UPLOAD_DIR/photos`, served publicly at `/api/files/photos/{name}` (img tags send
  no auth header); each upload gets a new random name, so files are cached forever.
- Prices are configuration, not code.

## Subscriptions and stats

- Prices come from `PRICE_BASIC_KZT` / `PRICE_FEATURED_KZT`; payment is manual (Kaspi), an admin
  activates. `months` extends after the running plan of the same plan; `ends_on` starts now.
- Worker: 7-day and 1-day reminders before a teacher's *last* plan ends, and one notice after it
  ended (only if it ended < 3 days ago). Flags on `subscriptions` make each message go out once.
- `profile_events`: `view` (not owner/admin), `search_impression` (each card on a search page),
  `contact`, `request`. `search_log` keeps first-page searches for the dashboard's unmet demand.
  Stats are aggregated on read (weeks start on Monday in `TIMEZONE`).

## Trust

- Reviews need an effective `completed` lesson with the teacher; one per student per teacher
  (UNIQUE), editable; hidden reviews leave the rating. `visibility.RatingSQL` feeds the card,
  the `rating_min` filter and ranking (after "free this week").
- Blocked users (`users.blocked`) get 403 on every authenticated endpoint (auth middleware).
  Blocked profiles keep `block_reason`; unblock returns them to live (if ever approved) or draft.
- Verification documents are never public: only `GET /api/admin/documents/{id}/file` (admins).

## Lesson rules

- Slots: start every 30 min inside a weekly window (`weekday` 0 = Sunday, `HH:MM`, 24:00 allowed),
  last the subject's `lesson_minutes`, start >= now + 1 h and <= 30 days ahead, not on a day off.
- New lessons are `pending` with `expires_at = min(now + HOLD_MINUTES, start)`. Teacher confirms ->
  `confirmed`, declines -> `declined`; the worker flips overdue pending -> `expired` and ended
  confirmed -> `completed`. Student cancels pending any time before the start, confirmed only until
  `specialists.cancel_hours` before; the teacher cancels any upcoming lesson.
- A lesson **blocks** time iff `lessonstatus.BlockingSQL`; every read reports `lessonstatus.EffectiveSQL`.
  Exclusion constraints `lessons_no_specialist_overlap` / `lessons_no_student_overlap` cover
  pending + confirmed rows.
- **Create-lesson transaction** (`lesson.Repo.Insert`): lock `specialists` row then the student's
  `users` row (always this order, FOR NO KEY UPDATE) -> `ExpireOverdueSQL` -> INSERT; map 23P01 to
  409 `slot_taken` / `student_busy`.
- Confirm/decline: `lesson.Service.Decide`, shared by HTTP and the bot callbacks `ls:c:<id>` / `ls:d:<id>`.
- The student sees the teacher's address once a lesson with them is confirmed.

## Workflow

- Stacked milestone branches (m0-foundation, m1-catalogue, ...), each PR into the previous one.
- Run `make lint`, `make test-db` and `npm run build` before pushing.
- PR description: summary, API contract changes, "How I tested" with real commands and results.
  Never merge.
- Never commit `.env`, bot tokens or other secrets.

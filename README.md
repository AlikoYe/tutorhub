# TutorHub

A catalogue of teachers, tutors and consultants in Kazakhstan, opened as a Telegram Mini App.
Students search by subject, level, language, format, district and price, and request a lesson
in two taps. Teachers pay a monthly subscription to be listed. See `PLAN.md`.

## Run locally

Requirements: Go 1.26, Node 22, Docker (or a local PostgreSQL 16).

```bash
cp .env.example .env
make dev          # Postgres in Docker, migrations, API on :8080, Vite on :5173
make seed         # optional demo data
```

Open http://localhost:5173. Outside Telegram the app logs in as `VITE_DEV_TELEGRAM_ID`
(dev auth, only when `APP_ENV=dev`).

Without Docker, point `DATABASE_URL` at any PostgreSQL 16 and run
`cd backend && go run ./cmd/migrate up && go run ./cmd/api` plus `cd frontend && npm run dev`.

## Test

```bash
make lint
make test-db      # all Go tests against real PostgreSQL (tutorhub_test)
cd frontend && npm run build
```

More in `CLAUDE.md`; production runbook in `docs/deploy.md`.

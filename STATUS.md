# Status

All milestones of `PLAN.md` are implemented, M0–M5. Each one is an open PR, stacked on the previous one. Nothing is merged.

## Milestones and PRs

| Milestone | Branch | PR (base) |
|---|---|---|
| M0 Repo foundation | `m0-foundation` | https://github.com/AlikoYe/tutorhub/pull/1 (`main`) |
| M1 Profiles and catalogue | `m1-catalogue` | https://github.com/AlikoYe/tutorhub/pull/2 (`m0-foundation`) |
| M2 Availability and lessons | `m2-lessons` | https://github.com/AlikoYe/tutorhub/pull/3 (`m1-catalogue`) |
| M3 Subscriptions and statistics | `m3-subscriptions` | https://github.com/AlikoYe/tutorhub/pull/4 (`m2-lessons`) |
| M4 Trust | `m4-trust` | https://github.com/AlikoYe/tutorhub/pull/5 (`m3-subscriptions`) |
| M5 Growth | `m5-growth` | https://github.com/AlikoYe/tutorhub/pull/6 (`m4-trust`) |

M0–M3 are the MVP. Review and merge in order (#1 first). After merging #1 into `main`, retarget #2 to `main`, and so on. The final state is also pushed to `claude/dazzling-mendel-36zi2o`.

Every PR passed, at its own head: gofmt and go vet clean, all Go tests with `-race` against a real PostgreSQL 16.15, every HTTP response in the tests validated against `docs/api.yaml`, `redocly lint` valid, and `npm run build`. Each PR description lists the real commands, results and manual checks.

## Unfinished, untested or worth knowing

- **Not run here** (no Docker daemon in this sandbox):
  - GitHub Actions themselves;
  - `docker compose` and the `make dev` / `make test-db` / `make up` targets, which use the compose Postgres;
  - `deploy/*.sh`;
  - shellcheck and actionlint.
  The workflows and compose files were adapted from TableBot and kept consistent, but they have not run.
- **The Telegram bot never talked to real Telegram** (no token, as required). Every bot path is tested with a fake `Sender`, and with an empty `BOT_TOKEN` the API logs messages instead of sending them.
- **Languages:** teacher and admin screens and all bot messages are Russian only. The student-facing Mini App screens and the public website are ru/kk/en.
- **Payments are manual** (Kaspi transfer, then an admin activates the plan), as planned for the MVP. `PAYMENT_INSTRUCTIONS` is a placeholder text. Prices are the plan's placeholders (2,990 / 7,990 KZT).
- **Frontend has no automated tests.** Only the build and Playwright screenshots of the main screens were checked.
- **Approximations:** "free this week" is a SQL approximation (see DECISIONS.md), and statistics are aggregated on read (there is no daily rollup table yet).
- **Not in any milestone:** online payment, a map view, recurring lessons, learning centres (the plan's "after M5").
- Every choice made without the owner is one line in `DECISIONS.md`.

## Run locally

Requirements: Go 1.26 (`GOTOOLCHAIN=auto` downloads it), Node 22, and Docker or a local PostgreSQL 16.

With Docker:

```bash
cp .env.example .env
make dev                 # Postgres in Docker, migrations, API :8080, Vite :5173
make seed ADMIN=1        # 8 demo Almaty teachers with hours, lessons and reviews; user 1 is admin
```

Then open http://localhost:5173. Outside Telegram the app logs in as `VITE_DEV_TELEGRAM_ID`, or as `localStorage.devTelegramId` (`APP_ENV=dev` only). The public site is at http://localhost:5173/c/almaty and http://localhost:5173/sitemap.xml.

Without Docker (any PostgreSQL 16 with the `btree_gist` and `pg_trgm` extensions available):

```bash
createdb tutorhub && createdb tutorhub_test
export DATABASE_URL=postgres://USER:PASS@localhost:5432/tutorhub?sslmode=disable
cd backend && go run ./cmd/migrate up
psql "$DATABASE_URL" -v admin_id=1 -f seed/dev.sql
APP_ENV=dev go run ./cmd/api            # API on :8080
cd ../frontend && npm ci && VITE_DEV_TELEGRAM_ID=1 npm run dev
```

Checks:

```bash
cd backend && gofmt -l . && go vet ./...
TEST_DATABASE_URL=postgres://USER:PASS@localhost:5432/tutorhub_test?sslmode=disable go test -race -count=1 -p 1 ./...
npx @redocly/cli lint docs/api.yaml
cd frontend && npm run build
```

Production runbook: `docs/deploy.md`. Conventions: `CLAUDE.md`.

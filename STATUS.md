# Status

All milestones of `PLAN.md` are implemented, M0–M5, plus the production deploy setup (M6). Each one is an open PR, stacked on the previous one. Nothing is merged.

## Milestones and PRs

| Milestone | Branch | PR (base) |
|---|---|---|
| M0 Repo foundation | `m0-foundation` | https://github.com/AlikoYe/tutorhub/pull/1 (`main`) |
| M1 Profiles and catalogue | `m1-catalogue` | https://github.com/AlikoYe/tutorhub/pull/2 (`m0-foundation`) |
| M2 Availability and lessons | `m2-lessons` | https://github.com/AlikoYe/tutorhub/pull/3 (`m1-catalogue`) |
| M3 Subscriptions and statistics | `m3-subscriptions` | https://github.com/AlikoYe/tutorhub/pull/4 (`m2-lessons`) |
| M4 Trust | `m4-trust` | https://github.com/AlikoYe/tutorhub/pull/5 (`m3-subscriptions`) |
| M5 Growth | `m5-growth` | https://github.com/AlikoYe/tutorhub/pull/6 (`m4-trust`) |
| M6 Production deploy (shared edge proxy) | `m6-deploy` | https://github.com/AlikoYe/tutorhub/pull/7 (`m5-growth`) |
| TableBot side of M6 | TableBot `edge-proxy` | https://github.com/AlikoYe/TableBot/pull/5 (`main`) |

M0–M3 are the MVP. Review and merge in order (#1 first). After merging #1 into `main`, retarget #2 to `main`, and so on. The final state is also pushed to `claude/dazzling-mendel-36zi2o`.

Every PR passed, at its own head: gofmt and go vet clean, all Go tests with `-race` against a real PostgreSQL 16.15, every HTTP response in the tests validated against `docs/api.yaml`, `redocly lint` valid, and `npm run build`. Each PR description lists the real commands, results and manual checks.

## Production deploy (M6)

TutorHub runs on TableBot's VM behind a shared edge proxy:
- one Caddy, project `edge` in `/opt/edge`, owns 80/443 and all certificates;
- TableBot's and TutorHub's web containers serve plain `:80` on the Docker network `edge` as `tablebot-web` and `tutorhub-web`.

The full runbook is `docs/deploy.md`.

### Cutover the owner must run (details and expected output in docs/deploy.md, step 4)

0. Point a domain at the VM (A record) or use `tutorhub.<ip-with-dashes>.sslip.io`. Create the production bot in BotFather. On the server, run `sudo bash tutorhub-setup.sh` and fill `/opt/tutorhub/.env`. Merge the TutorHub PRs to `main` first, or check out `m6-deploy` on the server.
1. a) `TABLEBOT_HOST=tablebot-g30.indiasouthcentral.cloudapp.azure.com TUTORHUB_HOST=<tutorhub-host> bash /opt/tutorhub/deploy/edge/install.sh --no-start`
2. b) Merge https://github.com/AlikoYe/TableBot/pull/5. TableBot auto-deploys and frees 80/443. **TableBot is down from here.**
3. c) Immediately run `bash /opt/tutorhub/deploy/edge/install.sh --copy-certs`. TableBot is back through the edge.
4. d) `cd /opt/tutorhub && bash deploy/deploy.sh --no-pull`. Optionally `make prod-seed ADMIN=888659722`. Then `/start` the bot (the menu button is set by the API) and run the health checks.
5. e) If c) fails: `bash /opt/tutorhub/deploy/edge/rollback-tablebot.sh` puts TableBot back on its own 80/443.
6. Add TutorHub's own restricted deploy key and the 4 GitHub secrets (docs/deploy.md, step 5).

### What was verified (no Docker daemon here)

- `caddy validate` / `caddy adapt` (caddy v2.10.2) pass for every Caddyfile: TutorHub's inner one, TableBot's patched inner one, and the edge one with generated site files (two hosts, one host, no hosts).
- `install.sh` ran for real against a stub `docker` CLI that maps validation to the local caddy:
  - `--no-start` writes the stack, both site files and `edge.env`;
  - a re-run with no variables reuses the saved hosts and runs the `--copy-certs` and start path;
  - `TUTORHUB_HOST=-` removes a site;
  - a malformed host is rejected.
- **End to end without Docker:**
  - the TutorHub API, TutorHub's real inner Caddyfile on :8081 serving the built frontend, TableBot's patched inner Caddyfile on :8082 (stub page), and the edge with the generated site files (rewritten to `http://` :9080 and `tls internal` :9443);
  - `curl --resolve` returned the TutorHub SPA, `/api/health`, the API search, the public teacher page, the sitemap and an SPA route for `tutorhub.test`, and the TableBot stub for `tablebot.test`;
  - the inner Caddy saw `X-Forwarded-Proto=https` from the edge, and a spoofed client `X-Forwarded-For` was ignored. Outputs are in PR #7.
- `docker compose config` (the CLI works without a daemon) passes for TutorHub base+prod, the edge stack, TableBot base+prod, and TableBot with the rollback overlay. In the resolved prod config no TutorHub service publishes a port, and the web container has `SITE_ADDRESS=:80` and the alias `tutorhub-web`.
- shellcheck is clean on every script in both repos, and actionlint is clean on both repos' workflows.
- All Go tests still pass (`-race`, PostgreSQL 16).

### Not verified

- Real `docker compose up`, the images, and the edge actually obtaining Let's Encrypt certificates.
- `--copy-certs` against a real `tablebot_caddy_data` volume.
- The GitHub deploy workflows over SSH.
- The bot against real Telegram.

These need the VM.

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

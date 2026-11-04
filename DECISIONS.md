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
- M1: one `specialist` package holds the owner editor, admin verification and catalogue search (they share the same rows and queries); `visibility` holds the listing rule like TableBot's `bookingstatus`.
- M1: the `subscriptions` table is created in M1 (not M3) because the visibility rule needs it; approval starts a `trial` (TRIAL_DAYS, default 30) or a `founding` plan until a date the admin sets.
- M1: editing a live profile does not send it back to moderation (fast edits of prices/times matter more than re-review in the MVP; admins can still block).
- M1: the profile page opens for anyone while `status = live`, even when the subscription has ended (the plan says the link keeps working); draft/pending/hidden/blocked are visible only to the owner and admins.
- M1: levels are primary (1-4), middle (5-9), high (10-11), ent, university, adult; formats online/teacher_place/student_home; languages kk/ru/en.
- M1: subject, level and price filters must match the same subject offer (e.g. "ENT math under 5000" does not match a teacher whose cheap subject is something else).
- M1: photos are public files under /api/files/photos (image tags cannot send auth headers); names are random so URLs are not guessable by id.
- M1: search returns `{items, total}` (not a bare array) so the client can page.

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
- M2: lesson starts snap to a 30-minute grid inside the teacher's windows and must be at least 1 hour ahead and at most 30 days ahead (simple, predictable slot lists).
- M2: lesson length is per subject (nullable override on specialist_subjects, default the profile's lesson_minutes); the teacher's cancel rule is `cancel_hours` on the profile (default 12 h).
- M2: weekday numbering follows Go/PostgreSQL DOW (0 = Sunday) to avoid conversions in SQL.
- M2: "has free slots this week" (filter + ranking) is one SQL approximation: a non-closed window in the next 7 local days that is not fully covered by blocking lessons; exact slot lists come from GET /specialists/{id}/slots.
- M2: a teacher whose subscription has ended can still be booked through their link (status live); only unlisted from search, matching the plan's "link still works for their own students".
- M2: confirmed lessons become `completed` by the worker after they end; reads report `completed` immediately (effective status), so reviews (M4) can rely on it.
- M2: reminders go to both the student and the teacher once, REMINDER_BEFORE (default 2 h) before a confirmed lesson.
- M2: favourites and the "contact" button are deferred to M3 because "contact" is counted as a lead in profile_events (M3).
- M3: subscription extension rule: `months` starts when the running/future subscription of the same plan ends (renewals stack), `ends_on` starts now (upgrades such as featured take effect immediately). Admins can end a subscription (a future one is deleted).
- M3: "set founding period" is one admin action that moves the end of every running founding plan to a date (POST /admin/founding-period).
- M3: expiry reminders consider only a teacher's last subscription (a renewed teacher is not reminded) and the "you left search" notice is sent only for plans that ended less than 3 days ago, so the first deploy does not message old accounts.
- M3: hiding on expiry needs no status change — the visibility rule already drops teachers without a running plan; the worker only notifies.
- M3: statistics are aggregated on read from append-only profile_events (indexed by specialist and time) instead of a daily rollup table; enough for MVP volumes, a rollup can come later.
- M3: views by the owner and by admins are not counted; impressions are counted for every card on any search page; searches are logged once per query (first page) for the dashboard.
- M3: "unmet demand" = subjects searched in the last 30 days with fewer than 3 listed specialists, fewest first.
- M3: favourites and the contact button were added here (contact = lead event); the contact link is https://t.me/<username> or tg://user?id=<id>.

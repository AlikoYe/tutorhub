# TutorHub — product and build plan

Working name; rename later. Read this whole file before writing code. Each milestone below is
one pull request: build it, test it, open a PR, stop and wait for review.

## 1. Product in one paragraph

A catalogue of teachers, tutors and consultants in Kazakhstan, opened as a **Telegram Mini App**
(and later as a public website). Students and parents search by subject, level, language, format,
district and price, see a teacher's profile, free time slots and reviews, and request a lesson in
two taps. Teachers manage requests, schedule and students inside Telegram.

**Business model:** free for students, no fees on lessons or payments. **Teachers pay a monthly
subscription to be visible in the catalogue.** The goal is to become *the* place where every
teacher in Kazakhstan is listed. Booking and the schedule are free tools that make the catalogue
useful; they are not what teachers pay for.

## 2. Users

| Role | Who | What they need |
|---|---|---|
| Student / parent | Anyone in Telegram | Find a trustworthy teacher fast; book a trial lesson; reminders |
| Teacher | Tutors: school subjects, ENT/UNT prep, IELTS, languages, programming, music... | Be found; get requests; manage schedule and students |
| Consultant | University admission, study abroad, career, exam strategy | Same as teacher, sells consultations and packages |
| Admin (us) | The team | Verify teachers, activate subscriptions, moderate reviews, see metrics |

Teacher and consultant are the same entity (`specialist`) with a `kind` field.

## 3. Go-to-market rules the product must support

1. **Founding period.** The first teachers get free visibility (a "founding" plan with an end
   date, set by admin) so the catalogue fills up before we charge. Without supply nobody pays.
2. **Paid visibility.** After the trial or founding period ends, a teacher without an active
   subscription disappears from search (profile kept, data kept, link still works for their own
   students). Renewing brings them back instantly.
3. **Proof of value.** Every teacher sees their numbers: profile views, appearances in search,
   requests, booked lessons, per week. This is what makes them renew.
4. **Featured placement** (higher plan) ranks a teacher above others in their subject and district.
5. **No commission, no per-student fees, no payment processing** in the MVP. Subscriptions are
   paid manually first (Kaspi transfer → admin activates), online payment later.

Prices are configuration, not code. Placeholders until validated with real teachers:
`basic` 2,990 KZT/month (listed), `featured` 7,990 KZT/month (listed + top ranking + badge).

## 4. Features

### Student (Mini App)
- **Catalogue search:** subject, level (school grade, ENT, university, adult), language
  (kazakh, russian, english), format (online, at teacher's place, at student's home),
  city and district, price range, "has free slots this week", rating. Text search by name/subject.
- **Teacher profile:** photo, short intro, subjects with levels and price per lesson, experience
  and education (verified badge when admin checked documents), languages, formats, district /
  address (exact address only after a booking is confirmed), weekly availability, reviews,
  "trial lesson" price or free trial.
- **Request a lesson:** pick a free slot → request (pending, held for a limited time) →
  teacher confirms or declines in Telegram → student is notified. Trial lesson flag.
- **My lessons:** upcoming / past, cancel within the teacher's rules, reminders before lessons.
- **Favourites** and "contact" (opens Telegram chat with the teacher; counted as a lead).
- **Reviews:** only after a completed lesson, one per student per teacher, editable.

### Teacher (Mini App + bot)
- **Onboarding in the bot:** `/teach` → opens the profile wizard. Profile is a draft until
  submitted; admin verifies; then it goes live (founding or trial plan).
- **Profile editor:** all profile fields, subjects with prices, photo upload.
- **Availability:** weekly template (e.g. Mon/Wed 16:00–20:00), lesson length per subject,
  exceptions (days off).
- **Requests** with ✅/❌ in the bot and in the app; the same double-booking protection as TableBot.
- **Students list:** everyone who booked, upcoming lessons, private notes.
- **Statistics:** views, search appearances, contacts, requests, lessons — per week.
- **Subscription page:** current plan, end date, how to pay, renewal reminders from the bot
  7 days and 1 day before expiry.
- **Personal link** `t.me/<bot>?start=t_<id>` to share with existing students.

### Admin (Mini App admin tab + bot commands)
- Queue of profiles waiting for verification (approve / reject with reason).
- Subscriptions: activate plan for N months, extend, set founding period, see who expires soon.
- Moderate reviews and reported profiles; block users.
- Dashboard: teachers by status, searches per day, requests per day, top subjects with no
  teachers (demand we cannot serve yet).

### Later (not in MVP)
Public website with SEO pages per teacher and per "subject + city" (big for organic growth),
map view, online payment of subscriptions, recurring lessons, learning centres with several
teachers, Kazakh/English UI polish, AI matching ("ENT math 120+, budget X").

## 5. Architecture

Same proven stack as TableBot (github.com/AlikoYe/TableBot, public):

- **Backend:** Go (chi, pgx/v5, goose, slog, go-telegram/bot), one process: HTTP API + bot +
  background worker. Modular monolith: `handler -> service -> repo` per domain package.
- **Database:** PostgreSQL 16. Exclusion constraints for lessons (no double booking per teacher
  and per student), same pattern as TableBot `bookings`.
- **Frontend:** Vue 3 + Vite, Telegram Mini App (theme variables, MainButton, BackButton).
- **Auth:** Telegram `initData` HMAC validation on every request; dev bypass only in `APP_ENV=dev`.
- **Files:** teacher photos and documents stored on disk volume (MVP), served via Caddy; S3-compatible later.
- **Infra:** Docker Compose + Caddy (auto HTTPS) on one VM; GitHub Actions: tests on PR,
  deploy on merge to `main`. Runbook in `docs/deploy.md`.
- **Contract first:** `docs/api.yaml` (OpenAPI 3.1); HTTP tests validate every response against it.

Reuse from TableBot (copy and adapt, do not import): `internal/auth`, `internal/platform/*`,
`internal/bot` sender/notifier pattern, `internal/notify` worker pattern, the booking engine
idea (`bookingstatus`, locking order, expire-before-insert), `deploy/*`, CI workflows, Makefile.
Write the frontend fresh in the same style (the TableBot frontend was written by another person).

### Data model (first version)

```
users            telegram_id PK, name, username, language_code, role (user|admin), created_at
specialists      id, user_id FK unique, kind (teacher|consultant), status (draft|pending|live|hidden|blocked),
                 display_name, photo_url, bio, education, experience_years, verified bool,
                 city, district, address, lat, lng, formats text[], languages text[],
                 trial_kind (none|free|paid), trial_price, lesson_minutes default 60,
                 created_at, updated_at
subjects         id, slug, name_ru, name_kk, name_en, category      (seeded taxonomy)
specialist_subjects specialist_id, subject_id, levels text[], price_kzt   PK(specialist_id, subject_id)
availability_rules  id, specialist_id, weekday 0-6, starts_at time, ends_at time
availability_exceptions specialist_id, day date, closed bool
lessons          id, specialist_id, student_id, subject_id, during tstzrange, status
                 (pending|confirmed|declined|cancelled|expired|completed), is_trial,
                 note, expires_at, created_at
                 EXCLUDE (specialist_id =, during &&) WHERE status IN (pending, confirmed)
                 EXCLUDE (student_id =, during &&)    WHERE status IN (pending, confirmed)
reviews          id, lesson_id unique, specialist_id, student_id, rating 1-5, text, status, created_at
favourites       user_id, specialist_id
subscriptions    id, specialist_id, plan (founding|trial|basic|featured), starts_at, ends_at,
                 note, created_by_admin
profile_events   specialist_id, kind (view|search_impression|contact|request), user_id null, at
                 (append-only; aggregated daily for stats)
```

**Visibility rule (one SQL helper, like TableBot `bookingstatus`):** a specialist appears in
search iff `status = 'live'` AND there is a subscription with `now()` between `starts_at` and
`ends_at`. Ranking: featured first, then has free slots soon, then rating, then newest.

## 6. Milestones (one PR each)

**M0 — Repo foundation.** New repo layout (backend/, frontend/, deploy/, docs/), `CLAUDE.md`
with conventions (adapted from TableBot), auth + platform + bot skeleton + worker skeleton,
migration with `users` + `subjects` seed (~40 subjects in ru/kk/en), health and `/me`,
`docs/api.yaml` v0, CI. Frontend shell with Telegram integration and empty screens.

**M1 — Specialist profiles and catalogue.** Profile CRUD (draft → submit → admin approve), photo
upload, subjects with prices, catalogue search with all filters + text search, profile page,
admin verification queue, bot `/teach`. Visibility rule implemented (founding/trial plans
created by admin for now). Tests: filters, visibility, permissions.

**M2 — Availability and lessons.** Weekly rules + exceptions, free slots API, lesson requests
with hold + confirm/decline in bot and app, cancel rules, reminders, double-booking protection
with the 20-parallel-requests test, "My lessons", teacher's students list.

**M3 — Subscriptions and statistics.** Plans, admin activation/extension, founding period,
expiry worker (hide from search when expired, reminders 7 d and 1 d before), featured ranking,
`profile_events` tracking + weekly stats screen for teachers, admin dashboard.

**M4 — Trust.** Reviews after completed lessons, rating on profile and in ranking, report
profile, block user, verified badge flow with document upload (documents visible to admins only).

**M5 — Growth.** Public website: server-rendered teacher pages and "subject + city" pages with
SEO tags and sitemap; share links; Kazakh and English UI.

After M5: online subscription payments, map view, recurring lessons.

## 7. Definition of done for every milestone

- gofmt + go vet clean, all tests pass against real PostgreSQL 16, HTTP responses validated
  against `docs/api.yaml`, `npm run build` passes.
- Telegram login works only through verified initData outside dev.
- No secrets committed. `.env.example` lists every variable.
- PR description: summary, contract changes, "How I tested" with real commands and results.

## 8. Open questions for the founders (do not block on them; use the defaults)

- Product name and bot username. Default: working name TutorHub.
- Exact prices and founding-period length. Default: placeholders above, founding until a date
  the admin sets.
- Cities at launch. Default: Almaty only; data model supports any city.

## Progress

- [x] M0 — Repo foundation (PR #1)
- [x] M1 — Specialist profiles and catalogue (PR #2)
- [x] M2 — Availability and lessons (PR #3)
- [x] M3 — Subscriptions and statistics (PR #4)
- [ ] M4 — Trust
- [ ] M5 — Growth

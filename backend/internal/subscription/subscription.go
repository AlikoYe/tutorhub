// Package subscription manages paid visibility: plans and prices, admin activation and
// extension, the founding period, and the expiry worker (reminders 7 and 1 days before the
// end, a notice when a teacher leaves search). Visibility itself is visibility.ListedSQL.
package subscription

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/notify"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
	"github.com/AlikoYe/tutorhub/backend/internal/visibility"
)

var (
	ErrNotFound = errors.New("subscription not found")
)

// InvalidError is a 400-class input error.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

// Plan is a purchasable plan.
type Plan struct {
	Plan     string `json:"plan"`
	PriceKZT int    `json:"price_kzt"`
}

// Plans is the price list shown to teachers.
type Plans struct {
	Plans               []Plan `json:"plans"`
	PaymentInstructions string `json:"payment_instructions"`
}

// State is a specialist's visibility and subscription history.
type State struct {
	SpecialistID   int64                     `json:"specialist_id"`
	Listed         bool                      `json:"listed"`
	Current        *specialist.Subscription  `json:"current"`
	CoverageEndsAt *time.Time                `json:"coverage_ends_at"` // end of the last running or future subscription
	History        []specialist.Subscription `json:"history"`
}

// Expiring is a specialist whose coverage ends soon.
type Expiring struct {
	SpecialistID int64            `json:"specialist_id"`
	DisplayName  string           `json:"display_name"`
	Owner        specialist.Owner `json:"owner"`
	Plan         string           `json:"plan"`
	EndsAt       time.Time        `json:"ends_at"`
}

// Notifier tells teachers about their plan. Implemented by bot.Notifier.
type Notifier interface {
	SubscriptionActivated(ctx context.Context, teacherID int64, s specialist.Subscription)
	SubscriptionEnding(ctx context.Context, teacherID int64, s specialist.Subscription)
	SubscriptionExpired(ctx context.Context, teacherID int64, s specialist.Subscription)
}

type Service struct {
	db       *pgxpool.Pool
	plans    Plans
	notifier Notifier
	loc      *time.Location
	log      *slog.Logger
	now      func() time.Time
}

func NewService(db *pgxpool.Pool, plans Plans, notifier Notifier, loc *time.Location, log *slog.Logger) *Service {
	return &Service{db: db, plans: plans, notifier: notifier, loc: loc, log: log, now: time.Now}
}

func (s *Service) Plans() Plans { return s.plans }

const subColumns = `sub.id, sub.plan, sub.starts_at, sub.ends_at, sub.note`

func scanSub(row pgx.Row) (specialist.Subscription, error) {
	var x specialist.Subscription
	err := row.Scan(&x.ID, &x.Plan, &x.StartsAt, &x.EndsAt, &x.Note)
	return x, err
}

func (s *Service) local(x specialist.Subscription) specialist.Subscription {
	x.StartsAt, x.EndsAt = x.StartsAt.In(s.loc), x.EndsAt.In(s.loc)
	return x
}

// State returns the specialist's subscriptions, newest end first.
func (s *Service) State(ctx context.Context, specialistID int64) (State, error) {
	st := State{SpecialistID: specialistID, History: []specialist.Subscription{}}
	err := s.db.QueryRow(ctx, `SELECT `+visibility.ListedSQL+` FROM specialists s WHERE s.id = $1`, specialistID).Scan(&st.Listed)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, specialist.ErrNotFound
	}
	if err != nil {
		return st, fmt.Errorf("select specialist: %w", err)
	}
	rows, err := s.db.Query(ctx, `SELECT `+subColumns+` FROM subscriptions sub
		WHERE sub.specialist_id = $1 ORDER BY sub.ends_at DESC, sub.id DESC LIMIT 50`, specialistID)
	if err != nil {
		return st, fmt.Errorf("select subscriptions: %w", err)
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (specialist.Subscription, error) { return scanSub(r) })
	if err != nil {
		return st, fmt.Errorf("scan subscriptions: %w", err)
	}
	now := s.now()
	for _, x := range list {
		x = s.local(x)
		st.History = append(st.History, x)
		running := !x.StartsAt.After(now) && x.EndsAt.After(now)
		if running && (st.Current == nil || x.Plan == visibility.PlanFeatured) {
			c := x
			st.Current = &c
		}
		if x.EndsAt.After(now) && (st.CoverageEndsAt == nil || x.EndsAt.After(*st.CoverageEndsAt)) {
			e := x.EndsAt
			st.CoverageEndsAt = &e
		}
	}
	return st, nil
}

// MyState is State for the teacher's own profile.
func (s *Service) MyState(ctx context.Context, repo *specialist.Repo, userID int64) (State, error) {
	id, err := repo.IDByUser(ctx, userID)
	if err != nil {
		return State{}, err
	}
	return s.State(ctx, id)
}

// ActivateInput adds a subscription. Exactly one of Months and EndsOn is set.
type ActivateInput struct {
	Plan    string
	Months  int
	EndsOn  *time.Time
	Note    string
	AdminID int64
}

// Activate adds a subscription. With Months it extends: it starts when the specialist's
// running or future subscription of the same plan ends (or now). With EndsOn it starts now and
// lasts through that local date.
func (s *Service) Activate(ctx context.Context, specialistID int64, in ActivateInput) (State, error) {
	in.Note = strings.TrimSpace(in.Note)
	switch {
	case in.Plan != visibility.PlanFounding && in.Plan != visibility.PlanTrial && in.Plan != visibility.PlanBasic && in.Plan != visibility.PlanFeatured:
		return State{}, &InvalidError{"plan must be founding, trial, basic or featured"}
	case (in.Months == 0) == (in.EndsOn == nil):
		return State{}, &InvalidError{"give exactly one of months (1-24) or ends_on"}
	case in.Months < 0 || in.Months > 24:
		return State{}, &InvalidError{"months must be between 1 and 24"}
	case utf8.RuneCountInString(in.Note) > 300:
		return State{}, &InvalidError{"note must be at most 300 characters"}
	}
	var teacherID int64
	if err := s.db.QueryRow(ctx, `SELECT user_id FROM specialists WHERE id = $1`, specialistID).Scan(&teacherID); errors.Is(err, pgx.ErrNoRows) {
		return State{}, specialist.ErrNotFound
	} else if err != nil {
		return State{}, fmt.Errorf("select specialist: %w", err)
	}

	now := s.now()
	start, end := now, time.Time{}
	if in.EndsOn != nil {
		end = specialist.EndOfDay(*in.EndsOn, s.loc)
		if !end.After(now) {
			return State{}, &InvalidError{"ends_on must be today or later"}
		}
	} else {
		var last *time.Time
		if err := s.db.QueryRow(ctx, `SELECT max(ends_at) FROM subscriptions WHERE specialist_id = $1 AND plan = $2 AND ends_at > now()`,
			specialistID, in.Plan).Scan(&last); err != nil {
			return State{}, fmt.Errorf("select coverage: %w", err)
		}
		if last != nil {
			start = *last
		}
		end = start.In(s.loc).AddDate(0, in.Months, 0)
	}

	x, err := scanSub(s.db.QueryRow(ctx, `
		INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at, note, created_by_admin)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+strings.ReplaceAll(subColumns, "sub.", ""),
		specialistID, in.Plan, start, end, in.Note, in.AdminID))
	if err != nil {
		return State{}, fmt.Errorf("insert subscription: %w", err)
	}
	s.notifier.SubscriptionActivated(ctx, teacherID, s.local(x))
	return s.State(ctx, specialistID)
}

// End stops a subscription now (or deletes it if it has not started).
func (s *Service) End(ctx context.Context, id int64) (State, error) {
	var specialistID int64
	err := s.db.QueryRow(ctx, `
		WITH gone AS (DELETE FROM subscriptions WHERE id = $1 AND starts_at > now() RETURNING specialist_id),
		     cut AS (UPDATE subscriptions SET ends_at = now() WHERE id = $1 AND starts_at <= now() AND ends_at > now() RETURNING specialist_id)
		SELECT specialist_id FROM gone UNION ALL SELECT specialist_id FROM cut
		UNION ALL SELECT specialist_id FROM subscriptions WHERE id = $1 AND ends_at <= now()
		LIMIT 1`, id).Scan(&specialistID)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{}, ErrNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("end subscription: %w", err)
	}
	return s.State(ctx, specialistID)
}

// SetFoundingPeriod moves the end of every running founding plan to the end of endsOn.
func (s *Service) SetFoundingPeriod(ctx context.Context, endsOn time.Time) (int, error) {
	end := specialist.EndOfDay(endsOn, s.loc)
	if !end.After(s.now()) {
		return 0, &InvalidError{"ends_on must be today or later"}
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE subscriptions SET ends_at = $1, reminded_7d = false, reminded_1d = false, expiry_notified = false
		WHERE plan = 'founding' AND starts_at <= now() AND ends_at > now()`, end)
	if err != nil {
		return 0, fmt.Errorf("set founding period: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Expiring lists live specialists whose coverage ends within days.
func (s *Service) Expiring(ctx context.Context, days int) ([]Expiring, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT ON (s.id) s.id, s.display_name, u.telegram_id, u.name, u.username, sub.plan, sub.ends_at
		FROM subscriptions sub
		JOIN specialists s ON s.id = sub.specialist_id
		JOIN users u ON u.telegram_id = s.user_id
		WHERE s.status = 'live' AND sub.ends_at > now() AND sub.ends_at <= now() + make_interval(days => $1)
		  AND NOT EXISTS (SELECT 1 FROM subscriptions o WHERE o.specialist_id = s.id AND o.ends_at > sub.ends_at)
		ORDER BY s.id, sub.ends_at DESC`, days)
	if err != nil {
		return nil, fmt.Errorf("select expiring: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Expiring, error) {
		var e Expiring
		err := r.Scan(&e.SpecialistID, &e.DisplayName, &e.Owner.TelegramID, &e.Owner.Name, &e.Owner.Username, &e.Plan, &e.EndsAt)
		e.EndsAt = e.EndsAt.In(s.loc)
		return e, err
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b Expiring) int { return a.EndsAt.Compare(b.EndsAt) })
	return out, nil
}

// lastOfCoverage keeps only a specialist's last subscription (no later one exists).
const lastOfCoverage = `NOT EXISTS (SELECT 1 FROM subscriptions o WHERE o.specialist_id = sub.specialist_id AND o.ends_at > sub.ends_at)`

const claim7d = `
	UPDATE subscriptions sub SET reminded_7d = true
	FROM specialists s
	WHERE s.id = sub.specialist_id AND s.status = 'live' AND NOT sub.reminded_7d
	  AND sub.starts_at <= now() AND sub.ends_at > now() + interval '1 day' AND sub.ends_at <= now() + interval '7 days'
	  AND ` + lastOfCoverage + `
	RETURNING sub.id, sub.plan, sub.starts_at, sub.ends_at, sub.note, s.user_id`

const claim1d = `
	UPDATE subscriptions sub SET reminded_1d = true, reminded_7d = true
	FROM specialists s
	WHERE s.id = sub.specialist_id AND s.status = 'live' AND NOT sub.reminded_1d
	  AND sub.starts_at <= now() AND sub.ends_at > now() AND sub.ends_at <= now() + interval '1 day'
	  AND ` + lastOfCoverage + `
	RETURNING sub.id, sub.plan, sub.starts_at, sub.ends_at, sub.note, s.user_id`

// Only recent expiries are announced, so the first deploy does not message long-gone teachers.
const claimExpired = `
	UPDATE subscriptions sub SET expiry_notified = true, reminded_1d = true, reminded_7d = true
	FROM specialists s
	WHERE s.id = sub.specialist_id AND s.status = 'live' AND NOT sub.expiry_notified
	  AND sub.ends_at <= now() AND sub.ends_at > now() - interval '3 days'
	  AND ` + lastOfCoverage + `
	RETURNING sub.id, sub.plan, sub.starts_at, sub.ends_at, sub.note, s.user_id`

func (s *Service) claim(ctx context.Context, sql string, send func(context.Context, int64, specialist.Subscription)) (int, error) {
	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return 0, err
	}
	type item struct {
		sub     specialist.Subscription
		teacher int64
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (item, error) {
		var it item
		err := r.Scan(&it.sub.ID, &it.sub.Plan, &it.sub.StartsAt, &it.sub.EndsAt, &it.sub.Note, &it.teacher)
		return it, err
	})
	if err != nil {
		return 0, err
	}
	for _, it := range items {
		send(ctx, it.teacher, s.local(it.sub))
	}
	return len(items), nil
}

// Jobs are the worker steps for subscriptions.
func (s *Service) Jobs() []notify.Job {
	return []notify.Job{
		notify.JobFunc{JobName: "subscriptions_remind_7d", Fn: func(ctx context.Context) (int, error) {
			return s.claim(ctx, claim7d, s.notifier.SubscriptionEnding)
		}},
		notify.JobFunc{JobName: "subscriptions_remind_1d", Fn: func(ctx context.Context) (int, error) {
			return s.claim(ctx, claim1d, s.notifier.SubscriptionEnding)
		}},
		notify.JobFunc{JobName: "subscriptions_expired", Fn: func(ctx context.Context) (int, error) {
			return s.claim(ctx, claimExpired, s.notifier.SubscriptionExpired)
		}},
	}
}

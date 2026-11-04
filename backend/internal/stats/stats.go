// Package stats records funnel events (profile views, search impressions, contacts, lesson
// requests) and turns them into the teacher's weekly numbers and the admin dashboard.
package stats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
	"github.com/AlikoYe/tutorhub/backend/internal/visibility"
)

// Values of profile_events.kind.
const (
	View             = "view"
	SearchImpression = "search_impression"
	Contact          = "contact"
	Request          = "request"
)

// Recorder writes events. Failures are logged and never fail the request.
type Recorder struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func NewRecorder(db *pgxpool.Pool, log *slog.Logger) *Recorder {
	return &Recorder{db: db, log: log}
}

// Record stores one event (lesson.Events).
func (r *Recorder) Record(ctx context.Context, specialistID int64, kind string, userID int64) {
	if _, err := r.db.Exec(ctx, `INSERT INTO profile_events (specialist_id, kind, user_id) VALUES ($1, $2, $3)`,
		specialistID, kind, nullID(userID)); err != nil {
		r.log.WarnContext(ctx, "record event", slog.String("kind", kind), slog.Int64("specialist_id", specialistID), slog.Any("err", err))
	}
}

// Viewed records a profile view (specialist.Events).
func (r *Recorder) Viewed(ctx context.Context, specialistID, userID int64) {
	r.Record(ctx, specialistID, View, userID)
}

// Searched logs a first-page search and an impression for every result shown (specialist.Events).
func (r *Recorder) Searched(ctx context.Context, userID int64, f specialist.Filter, page specialist.Page) {
	ids := make([]int64, 0, len(page.Items))
	for _, c := range page.Items {
		ids = append(ids, c.ID)
	}
	if len(ids) > 0 {
		if _, err := r.db.Exec(ctx, `INSERT INTO profile_events (specialist_id, kind, user_id)
			SELECT id, 'search_impression', $2 FROM unnest($1::bigint[]) AS id`, ids, nullID(userID)); err != nil {
			r.log.WarnContext(ctx, "record impressions", slog.Any("err", err))
		}
	}
	if f.Offset != 0 {
		return
	}
	var subject *int64
	if f.SubjectID != 0 {
		subject = &f.SubjectID
	}
	if _, err := r.db.Exec(ctx, `INSERT INTO search_log (user_id, subject_id, district, query, results) VALUES ($1, $2, $3, $4, $5)`,
		nullID(userID), subject, f.District, f.Query, page.Total); err != nil {
		r.log.WarnContext(ctx, "log search", slog.Any("err", err))
	}
}

func nullID(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

// Week is one week of a teacher's numbers (weeks start on Monday, app time zone).
type Week struct {
	WeekStart         string `json:"week_start"`
	Views             int    `json:"views"`
	SearchImpressions int    `json:"search_impressions"`
	Contacts          int    `json:"contacts"`
	Requests          int    `json:"requests"`
	Lessons           int    `json:"lessons"` // confirmed or completed lessons starting that week
}

// Teacher is the teacher's statistics screen.
type Teacher struct {
	Weeks  []Week `json:"weeks"` // oldest first; the last one is the current week
	Totals Week   `json:"totals"`
}

type Service struct {
	db  *pgxpool.Pool
	tz  string
	loc *time.Location
}

func NewService(db *pgxpool.Pool, loc *time.Location) *Service {
	return &Service{db: db, tz: loc.String(), loc: loc}
}

// TeacherStats returns the last `weeks` weeks for the specialist.
func (s *Service) TeacherStats(ctx context.Context, specialistID int64, weeks int) (Teacher, error) {
	rows, err := s.db.Query(ctx, `
		WITH w AS (
			SELECT d::date AS week, (d AT TIME ZONE $3) AS from_at, ((d + interval '7 days') AT TIME ZONE $3) AS to_at
			FROM generate_series(date_trunc('week', now() AT TIME ZONE $3) - make_interval(weeks => $2 - 1),
			                     date_trunc('week', now() AT TIME ZONE $3), interval '7 days') AS d
		)
		SELECT w.week,
		       count(e.id) FILTER (WHERE e.kind = 'view'),
		       count(e.id) FILTER (WHERE e.kind = 'search_impression'),
		       count(e.id) FILTER (WHERE e.kind = 'contact'),
		       count(e.id) FILTER (WHERE e.kind = 'request'),
		       (SELECT count(*) FROM lessons l WHERE l.specialist_id = $1 AND l.status IN ('confirmed', 'completed')
		          AND lower(l.during) >= w.from_at AND lower(l.during) < w.to_at)
		FROM w LEFT JOIN profile_events e ON e.specialist_id = $1 AND e.at >= w.from_at AND e.at < w.to_at
		GROUP BY w.week, w.from_at, w.to_at
		ORDER BY w.week`, specialistID, weeks, s.tz)
	if err != nil {
		return Teacher{}, fmt.Errorf("select stats: %w", err)
	}
	out := Teacher{Weeks: []Week{}}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Week, error) {
		var wk Week
		var d time.Time
		err := r.Scan(&d, &wk.Views, &wk.SearchImpressions, &wk.Contacts, &wk.Requests, &wk.Lessons)
		wk.WeekStart = d.Format(time.DateOnly)
		return wk, err
	})
	if err != nil {
		return Teacher{}, fmt.Errorf("scan stats: %w", err)
	}
	out.Weeks = list
	for _, wk := range list {
		out.Totals.Views += wk.Views
		out.Totals.SearchImpressions += wk.SearchImpressions
		out.Totals.Contacts += wk.Contacts
		out.Totals.Requests += wk.Requests
		out.Totals.Lessons += wk.Lessons
	}
	if len(list) > 0 {
		out.Totals.WeekStart = list[0].WeekStart
	}
	return out, nil
}

// DayCount is a number per local date.
type DayCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// Demand is a subject people search for with too few listed specialists.
type Demand struct {
	SubjectID int64  `json:"subject_id"`
	Slug      string `json:"slug"`
	NameRU    string `json:"name_ru"`
	Searches  int    `json:"searches"`
	Listed    int    `json:"listed_specialists"`
}

// Dashboard is the admin overview.
type Dashboard struct {
	Users               int            `json:"users"`
	SpecialistsByStatus map[string]int `json:"specialists_by_status"`
	Listed              int            `json:"listed"`
	PlansRunning        map[string]int `json:"plans_running"`
	Expiring7d          int            `json:"expiring_7d"`
	SearchesPerDay      []DayCount     `json:"searches_per_day"`
	RequestsPerDay      []DayCount     `json:"requests_per_day"`
	UnmetDemand         []Demand       `json:"unmet_demand"`
}

// Dashboard builds the admin overview over the last `days` days.
func (s *Service) Dashboard(ctx context.Context, days int) (Dashboard, error) {
	d := Dashboard{
		SpecialistsByStatus: map[string]int{visibility.Draft: 0, visibility.Pending: 0, visibility.Live: 0, visibility.Hidden: 0, visibility.Blocked: 0},
		PlansRunning:        map[string]int{visibility.PlanFounding: 0, visibility.PlanTrial: 0, visibility.PlanBasic: 0, visibility.PlanFeatured: 0},
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&d.Users); err != nil {
		return d, fmt.Errorf("count users: %w", err)
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM specialists s WHERE `+visibility.ListedSQL).Scan(&d.Listed); err != nil {
		return d, fmt.Errorf("count listed: %w", err)
	}
	if err := countInto(ctx, s.db, d.SpecialistsByStatus, `SELECT status, count(*) FROM specialists GROUP BY status`); err != nil {
		return d, err
	}
	if err := countInto(ctx, s.db, d.PlansRunning, `SELECT plan, count(DISTINCT specialist_id) FROM subscriptions
		WHERE starts_at <= now() AND ends_at > now() GROUP BY plan`); err != nil {
		return d, err
	}
	if err := s.db.QueryRow(ctx, `SELECT count(DISTINCT s.id) FROM subscriptions sub JOIN specialists s ON s.id = sub.specialist_id
		WHERE s.status = 'live' AND sub.ends_at > now() AND sub.ends_at <= now() + interval '7 days'
		  AND NOT EXISTS (SELECT 1 FROM subscriptions o WHERE o.specialist_id = s.id AND o.ends_at > sub.ends_at)`).Scan(&d.Expiring7d); err != nil {
		return d, fmt.Errorf("count expiring: %w", err)
	}
	var err error
	if d.SearchesPerDay, err = s.perDay(ctx, `search_log`, `true`, days); err != nil {
		return d, err
	}
	if d.RequestsPerDay, err = s.perDay(ctx, `profile_events`, `kind = 'request'`, days); err != nil {
		return d, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT sj.id, sj.slug, sj.name_ru, count(*) AS searches,
		       (SELECT count(*) FROM specialist_subjects ss JOIN specialists s ON s.id = ss.specialist_id
		        WHERE ss.subject_id = sj.id AND `+visibility.ListedSQL+`) AS listed
		FROM search_log l JOIN subjects sj ON sj.id = l.subject_id
		WHERE l.at > now() - make_interval(days => 30)
		GROUP BY sj.id
		HAVING (SELECT count(*) FROM specialist_subjects ss JOIN specialists s ON s.id = ss.specialist_id
		        WHERE ss.subject_id = sj.id AND `+visibility.ListedSQL+`) < 3
		ORDER BY listed, searches DESC, sj.position
		LIMIT 10`)
	if err != nil {
		return d, fmt.Errorf("select demand: %w", err)
	}
	d.UnmetDemand, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Demand, error) {
		var x Demand
		err := r.Scan(&x.SubjectID, &x.Slug, &x.NameRU, &x.Searches, &x.Listed)
		return x, err
	})
	if err != nil {
		return d, fmt.Errorf("scan demand: %w", err)
	}
	return d, nil
}

func countInto(ctx context.Context, db *pgxpool.Pool, m map[string]int, sql string) error {
	rows, err := db.Query(ctx, sql)
	if err != nil {
		return fmt.Errorf("count: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		m[k] = n
	}
	return rows.Err()
}

// perDay counts rows of table matching where per local date over the last `days` days (zeros included).
func (s *Service) perDay(ctx context.Context, table, where string, days int) ([]DayCount, error) {
	rows, err := s.db.Query(ctx, `
		SELECT d::date, (SELECT count(*) FROM `+table+` t WHERE `+where+`
		                 AND t.at >= (d AT TIME ZONE $1) AND t.at < ((d + interval '1 day') AT TIME ZONE $1))
		FROM generate_series(date_trunc('day', now() AT TIME ZONE $1) - make_interval(days => $2 - 1),
		                     date_trunc('day', now() AT TIME ZONE $1), interval '1 day') AS d
		ORDER BY d`, s.tz, days)
	if err != nil {
		return nil, fmt.Errorf("per day %s: %w", table, err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (DayCount, error) {
		var x DayCount
		var d time.Time
		err := r.Scan(&d, &x.Count)
		x.Date = d.Format(time.DateOnly)
		return x, err
	})
}

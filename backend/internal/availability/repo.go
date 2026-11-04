package availability

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/lessonstatus"
)

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

// Schedule loads the weekly rules and the days off from `from` on.
func (r *Repo) Schedule(ctx context.Context, specialistID int64, from time.Time) (Schedule, error) {
	s := Schedule{Rules: []Rule{}, Closed: map[string]bool{}}
	rows, err := r.db.Query(ctx, `
		SELECT weekday, (extract(epoch FROM starts_at) / 60)::int, (extract(epoch FROM ends_at) / 60)::int
		FROM availability_rules WHERE specialist_id = $1 ORDER BY weekday, starts_at`, specialistID)
	if err != nil {
		return s, fmt.Errorf("select rules: %w", err)
	}
	s.Rules, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Rule, error) {
		var r Rule
		var wd int16
		err := row.Scan(&wd, &r.Start, &r.End)
		r.Weekday = time.Weekday(wd)
		return r, err
	})
	if err != nil {
		return s, fmt.Errorf("scan rules: %w", err)
	}
	rows, err = r.db.Query(ctx, `SELECT day FROM availability_exceptions
		WHERE specialist_id = $1 AND closed AND day >= $2::date ORDER BY day`, specialistID, from.Format(time.DateOnly))
	if err != nil {
		return s, fmt.Errorf("select days off: %w", err)
	}
	days, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return s, fmt.Errorf("scan days off: %w", err)
	}
	for _, d := range days {
		s.Closed[d.Format(time.DateOnly)] = true
	}
	return s, nil
}

// ReplaceRules swaps the weekly template in one transaction.
func (r *Repo) ReplaceRules(ctx context.Context, specialistID int64, rules []Rule) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM availability_rules WHERE specialist_id = $1`, specialistID); err != nil {
			return fmt.Errorf("delete rules: %w", err)
		}
		for _, rl := range rules {
			if _, err := tx.Exec(ctx, `INSERT INTO availability_rules (specialist_id, weekday, starts_at, ends_at)
				VALUES ($1, $2, $3::time, $4::time)`, specialistID, int(rl.Weekday), FormatClock(rl.Start), FormatClock(rl.End)); err != nil {
				return fmt.Errorf("insert rule: %w", err)
			}
		}
		return nil
	})
}

func (r *Repo) SetDayOff(ctx context.Context, specialistID int64, day time.Time, closed bool) error {
	var err error
	if closed {
		_, err = r.db.Exec(ctx, `INSERT INTO availability_exceptions (specialist_id, day, closed) VALUES ($1, $2::date, true)
			ON CONFLICT (specialist_id, day) DO UPDATE SET closed = true`, specialistID, day.Format(time.DateOnly))
	} else {
		_, err = r.db.Exec(ctx, `DELETE FROM availability_exceptions WHERE specialist_id = $1 AND day = $2::date`,
			specialistID, day.Format(time.DateOnly))
	}
	if err != nil {
		return fmt.Errorf("set day off: %w", err)
	}
	return nil
}

// Busy returns the specialist's blocking lessons overlapping [from, to).
func (r *Repo) Busy(ctx context.Context, specialistID int64, from, to time.Time) ([]Interval, error) {
	rows, err := r.db.Query(ctx, `SELECT lower(l.during), upper(l.during) FROM lessons l
		WHERE l.specialist_id = $1 AND `+lessonstatus.BlockingSQL+` AND l.during && tstzrange($2, $3)`, specialistID, from, to)
	if err != nil {
		return nil, fmt.Errorf("select busy: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Interval, error) {
		var i Interval
		err := row.Scan(&i.Start, &i.End)
		return i, err
	})
}

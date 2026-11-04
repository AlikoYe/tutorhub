package lesson

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/lessonstatus"
)

type Repo struct {
	db  *pgxpool.Pool
	loc *time.Location
}

func NewRepo(db *pgxpool.Pool, loc *time.Location) *Repo {
	return &Repo{db: db, loc: loc}
}

// Bookable is what a lesson request needs to know about the specialist and subject.
type Bookable struct {
	SpecialistID  int64
	UserID        int64
	Status        string
	TrialKind     string
	TrialPrice    *int
	PriceKZT      int
	LessonMinutes int
}

// Bookable loads the specialist and the offer for subjectID.
func (r *Repo) Bookable(ctx context.Context, specialistID, subjectID int64) (Bookable, error) {
	var b Bookable
	err := r.db.QueryRow(ctx, `SELECT id, user_id, status, trial_kind, trial_price FROM specialists WHERE id = $1`, specialistID).
		Scan(&b.SpecialistID, &b.UserID, &b.Status, &b.TrialKind, &b.TrialPrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrSpecialistGone
	}
	if err != nil {
		return b, fmt.Errorf("select specialist: %w", err)
	}
	err = r.db.QueryRow(ctx, `SELECT ss.price_kzt, coalesce(ss.lesson_minutes, s.lesson_minutes)
		FROM specialist_subjects ss JOIN specialists s ON s.id = ss.specialist_id
		WHERE ss.specialist_id = $1 AND ss.subject_id = $2`, specialistID, subjectID).Scan(&b.PriceKZT, &b.LessonMinutes)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrSubjectNotOffered
	}
	if err != nil {
		return b, fmt.Errorf("select offer: %w", err)
	}
	return b, nil
}

// HadTrial reports whether the student already has a live or past trial with the specialist.
func (r *Repo) HadTrial(ctx context.Context, specialistID, studentID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM lessons l
		WHERE l.specialist_id = $1 AND l.student_id = $2 AND l.is_trial
		  AND `+lessonstatus.EffectiveSQL+` IN ('pending', 'confirmed', 'completed'))`, specialistID, studentID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("select trial: %w", err)
	}
	return ok, nil
}

type insertRow struct {
	SpecialistID, StudentID, SubjectID int64
	StartsAt                           time.Time
	Minutes                            int
	ExpiresAt                          time.Time
	IsTrial                            bool
	PriceKZT                           int
	Note                               string
}

// Insert expires overdue requests of the teacher or student, then inserts a pending lesson, in one transaction.
func (r *Repo) Insert(ctx context.Context, in insertRow) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		// Lock specialist then student (always this order) so racing inserts queue instead of
		// deadlocking inside the exclusion constraints; the loser then gets a clean 23P01.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM specialists WHERE id = $1 FOR NO KEY UPDATE`, in.SpecialistID); err != nil {
			return fmt.Errorf("lock specialist: %w", err)
		}
		if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE telegram_id = $1 FOR NO KEY UPDATE`, in.StudentID); err != nil {
			return fmt.Errorf("lock student: %w", err)
		}
		if _, err := tx.Exec(ctx, lessonstatus.ExpireOverdueSQL, in.SpecialistID, in.StudentID); err != nil {
			return fmt.Errorf("expire overdue requests: %w", err)
		}
		return tx.QueryRow(ctx, `
			INSERT INTO lessons (specialist_id, student_id, subject_id, during, status, expires_at, is_trial, price_kzt, note)
			VALUES ($1, $2, $3, tstzrange($4::timestamptz, $4::timestamptz + make_interval(mins => $5), '[)'),
			        'pending', $6, $7, $8, $9)
			RETURNING id`,
			in.SpecialistID, in.StudentID, in.SubjectID, in.StartsAt, in.Minutes, in.ExpiresAt, in.IsTrial, in.PriceKZT, in.Note,
		).Scan(&id)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
		switch pgErr.ConstraintName {
		case "lessons_no_specialist_overlap":
			return 0, ErrSlotTaken
		case "lessons_no_student_overlap":
			return 0, ErrStudentBusy
		}
	}
	if err != nil {
		return 0, fmt.Errorf("insert lesson: %w", err)
	}
	return id, nil
}

const selectLesson = `
	SELECT l.id, ` + lessonstatus.EffectiveSQL + `, lower(l.during), upper(l.during), l.expires_at, l.is_trial,
	       l.price_kzt, l.note, l.cancelled_by, l.cancel_reason, l.created_at,
	       sj.id, sj.slug, sj.name_ru, sj.name_kk, sj.name_en,
	       s.id, s.display_name, s.photo_url, s.address, s.user_id, s.cancel_hours,
	       u.telegram_id, u.name, u.username
	FROM lessons l
	JOIN subjects sj ON sj.id = l.subject_id
	JOIN specialists s ON s.id = l.specialist_id
	JOIN users u ON u.telegram_id = l.student_id`

func (r *Repo) scan(row pgx.Row) (Lesson, error) {
	var l Lesson
	err := row.Scan(&l.ID, &l.Status, &l.StartsAt, &l.EndsAt, &l.ExpiresAt, &l.IsTrial,
		&l.PriceKZT, &l.Note, &l.CancelledBy, &l.CancelReason, &l.CreatedAt,
		&l.Subject.ID, &l.Subject.Slug, &l.Subject.NameRU, &l.Subject.NameKK, &l.Subject.NameEN,
		&l.Specialist.ID, &l.Specialist.DisplayName, &l.Specialist.PhotoURL, &l.address, &l.TeacherUserID, &l.CancelHours,
		&l.StudentInfo.TelegramID, &l.StudentInfo.Name, &l.StudentInfo.Username)
	if err != nil {
		return Lesson{}, err
	}
	l.StartsAt, l.EndsAt = l.StartsAt.In(r.loc), l.EndsAt.In(r.loc)
	l.CreatedAt = l.CreatedAt.In(r.loc).Truncate(time.Second)
	if l.Status == lessonstatus.Pending && l.ExpiresAt != nil {
		t := l.ExpiresAt.In(r.loc).Truncate(time.Second)
		l.ExpiresAt = &t
	} else {
		l.ExpiresAt = nil
	}
	return l, nil
}

func (r *Repo) Get(ctx context.Context, id int64) (Lesson, error) {
	l, err := r.scan(r.db.QueryRow(ctx, selectLesson+` WHERE l.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Lesson{}, ErrNotFound
	}
	if err != nil {
		return Lesson{}, fmt.Errorf("select lesson: %w", err)
	}
	return l, nil
}

func (r *Repo) list(ctx context.Context, sql string, args ...any) ([]Lesson, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("select lessons: %w", err)
	}
	defer rows.Close()
	out := []Lesson{}
	for rows.Next() {
		l, err := r.scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Scopes of lesson lists.
const (
	ScopeUpcoming = "upcoming" // pending or confirmed, not ended; soonest first
	ScopePast     = "past"     // everything else; newest first
	ScopeRequests = "requests" // pending only (teacher); soonest first
)

func scopeSQL(scope string) string {
	eff := lessonstatus.EffectiveSQL
	switch scope {
	case ScopeRequests:
		return ` AND ` + eff + ` = 'pending' ORDER BY lower(l.during), l.id`
	case ScopePast:
		return ` AND NOT (` + eff + ` IN ('pending', 'confirmed')) ORDER BY lower(l.during) DESC, l.id DESC`
	default:
		return ` AND ` + eff + ` IN ('pending', 'confirmed') ORDER BY lower(l.during), l.id`
	}
}

func (r *Repo) ListForStudent(ctx context.Context, studentID int64, scope string, limit int) ([]Lesson, error) {
	return r.list(ctx, selectLesson+` WHERE l.student_id = $1`+scopeSQL(scope)+` LIMIT $2`, studentID, limit)
}

func (r *Repo) ListForSpecialist(ctx context.Context, specialistID int64, scope string, limit int) ([]Lesson, error) {
	return r.list(ctx, selectLesson+` WHERE l.specialist_id = $1`+scopeSQL(scope)+` LIMIT $2`, specialistID, limit)
}

// Decide confirms or declines a live request; false when it is no longer pending.
func (r *Repo) Decide(ctx context.Context, id int64, confirm bool) (bool, error) {
	status := lessonstatus.Declined
	if confirm {
		status = lessonstatus.Confirmed
	}
	tag, err := r.db.Exec(ctx, lessonstatus.DecideSQL, id, status)
	if err != nil {
		return false, fmt.Errorf("decide lesson: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Cancel cancels a blocking lesson that has not started. A student may cancel a confirmed
// lesson only until cancel_hours before the start.
func (r *Repo) Cancel(ctx context.Context, id int64, by, reason string) (bool, error) {
	rule := ``
	if by == ByStudent {
		rule = ` AND (l.status = 'pending' OR lower(l.during) - now() >= make_interval(hours => s.cancel_hours))`
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE lessons l SET status = 'cancelled', cancelled_by = $2, cancel_reason = $3, updated_at = now()
		FROM specialists s
		WHERE l.id = $1 AND s.id = l.specialist_id AND lower(l.during) > now() AND `+lessonstatus.BlockingSQL+rule,
		id, by, reason)
	if err != nil {
		return false, fmt.Errorf("cancel lesson: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Students aggregates everyone who requested a lesson from the specialist.
func (r *Repo) Students(ctx context.Context, specialistID int64) ([]Student, error) {
	eff := lessonstatus.EffectiveSQL
	rows, err := r.db.Query(ctx, `
		SELECT u.telegram_id, u.name, u.username,
		       count(*) FILTER (WHERE `+eff+` IN ('confirmed', 'completed')),
		       count(*),
		       min(lower(l.during)) FILTER (WHERE `+eff+` = 'confirmed' AND lower(l.during) > now()),
		       max(lower(l.during)) FILTER (WHERE `+eff+` = 'completed'),
		       coalesce(n.note, '')
		FROM lessons l
		JOIN users u ON u.telegram_id = l.student_id
		LEFT JOIN student_notes n ON n.specialist_id = l.specialist_id AND n.student_id = l.student_id
		WHERE l.specialist_id = $1
		GROUP BY u.telegram_id, u.name, u.username, n.note
		ORDER BY 6 NULLS LAST, 7 DESC NULLS LAST, u.name`, specialistID)
	if err != nil {
		return nil, fmt.Errorf("select students: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Student, error) {
		var s Student
		err := row.Scan(&s.TelegramID, &s.Name, &s.Username, &s.Lessons, &s.Requests, &s.NextLessonAt, &s.LastLessonAt, &s.Note)
		if s.NextLessonAt != nil {
			t := s.NextLessonAt.In(r.loc)
			s.NextLessonAt = &t
		}
		if s.LastLessonAt != nil {
			t := s.LastLessonAt.In(r.loc)
			s.LastLessonAt = &t
		}
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan students: %w", err)
	}
	return out, nil
}

// SetNote stores the teacher's note about a student who has requested a lesson; false otherwise.
func (r *Repo) SetNote(ctx context.Context, specialistID, studentID int64, note string) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		INSERT INTO student_notes (specialist_id, student_id, note)
		SELECT $1, $2, $3 WHERE EXISTS (SELECT 1 FROM lessons WHERE specialist_id = $1 AND student_id = $2)
		ON CONFLICT (specialist_id, student_id) DO UPDATE SET note = EXCLUDED.note, updated_at = now()`,
		specialistID, studentID, note)
	if err != nil {
		return false, fmt.Errorf("set note: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// HasConfirmed reports whether the student has a confirmed or completed lesson with the specialist.
func (r *Repo) HasConfirmed(ctx context.Context, specialistID, studentID int64) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM lessons l
		WHERE l.specialist_id = $1 AND l.student_id = $2 AND l.status IN ('confirmed', 'completed'))`, specialistID, studentID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("select confirmed: %w", err)
	}
	return ok, nil
}

// Claim runs a worker UPDATE ... RETURNING id statement.
func (r *Repo) Claim(ctx context.Context, sql string, args ...any) ([]int64, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

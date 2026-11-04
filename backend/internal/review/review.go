// Package review lets a student rate a teacher after a completed lesson (one editable review
// per student per teacher) and lets admins hide reviews.
package review

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlikoYe/tutorhub/backend/internal/lessonstatus"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
)

var (
	ErrNotFound          = errors.New("review not found")
	ErrNoCompletedLesson = errors.New("you can review a teacher only after a completed lesson with them")
)

// InvalidError is a 400-class input error.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

// Review is a published review as everyone sees it.
type Review struct {
	ID         int64     `json:"id"`
	Rating     int       `json:"rating"`
	Text       string    `json:"text"`
	AuthorName string    `json:"author_name"` // first name only
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Mine is the student's own review state for a teacher.
type Mine struct {
	CanReview bool    `json:"can_review"`
	Review    *Review `json:"review"`
	Hidden    bool    `json:"hidden"` // hidden by a moderator
}

// AdminReview is a review in the moderation list.
type AdminReview struct {
	Review
	Status     string                   `json:"status"`
	Specialist specialist.SpecialistRef `json:"specialist"`
	StudentID  int64                    `json:"student_id"`
}

// Notifier tells the teacher about a new review. Implemented by bot.Notifier.
type Notifier interface {
	ReviewPosted(ctx context.Context, teacherID int64, specialistName string, rating int, text string)
}

// Profiles opens profiles with the specialist visibility rules.
type Profiles interface {
	Profile(ctx context.Context, v specialist.Viewer, id int64) (specialist.Public, error)
}

type Service struct {
	db       *pgxpool.Pool
	profiles Profiles
	notifier Notifier
	loc      *time.Location
	log      *slog.Logger
}

func NewService(db *pgxpool.Pool, profiles Profiles, notifier Notifier, loc *time.Location, log *slog.Logger) *Service {
	return &Service{db: db, profiles: profiles, notifier: notifier, loc: loc, log: log}
}

const reviewColumns = `r.id, r.rating, r.text, split_part(u.name, ' ', 1), r.created_at, r.updated_at`

func (s *Service) scan(row pgx.Row, extra ...any) (Review, error) {
	var x Review
	err := row.Scan(append([]any{&x.ID, &x.Rating, &x.Text, &x.AuthorName, &x.CreatedAt, &x.UpdatedAt}, extra...)...)
	x.CreatedAt, x.UpdatedAt = x.CreatedAt.In(s.loc).Truncate(time.Second), x.UpdatedAt.In(s.loc).Truncate(time.Second)
	return x, err
}

// List returns a visible specialist's published reviews, newest first.
func (s *Service) List(ctx context.Context, v specialist.Viewer, specialistID int64) ([]Review, error) {
	if _, err := s.profiles.Profile(ctx, v, specialistID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT `+reviewColumns+` FROM reviews r JOIN users u ON u.telegram_id = r.student_id
		WHERE r.specialist_id = $1 AND r.status = 'published' ORDER BY r.created_at DESC LIMIT 100`, specialistID)
	if err != nil {
		return nil, fmt.Errorf("select reviews: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Review, error) { return s.scan(r) })
}

func (s *Service) completedLesson(ctx context.Context, specialistID, studentID int64) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `SELECT l.id FROM lessons l
		WHERE l.specialist_id = $1 AND l.student_id = $2 AND `+lessonstatus.EffectiveSQL+` = 'completed'
		ORDER BY lower(l.during) DESC LIMIT 1`, specialistID, studentID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoCompletedLesson
	}
	return id, err
}

// Mine returns whether the student may review and their current review.
func (s *Service) Mine(ctx context.Context, v specialist.Viewer, specialistID int64) (Mine, error) {
	if _, err := s.profiles.Profile(ctx, v, specialistID); err != nil {
		return Mine{}, err
	}
	var m Mine
	var status string
	x, err := s.scan(s.db.QueryRow(ctx, `SELECT `+reviewColumns+`, r.status FROM reviews r JOIN users u ON u.telegram_id = r.student_id
		WHERE r.specialist_id = $1 AND r.student_id = $2`, specialistID, v.UserID), &status)
	switch {
	case err == nil:
		m.Review, m.Hidden, m.CanReview = &x, status == "hidden", true
		return m, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Mine{}, fmt.Errorf("select my review: %w", err)
	}
	_, err = s.completedLesson(ctx, specialistID, v.UserID)
	if err != nil && !errors.Is(err, ErrNoCompletedLesson) {
		return Mine{}, err
	}
	m.CanReview = err == nil
	return m, nil
}

// Save creates or updates the student's review of the teacher.
func (s *Service) Save(ctx context.Context, v specialist.Viewer, specialistID int64, rating int, text string) (Review, error) {
	text = strings.TrimSpace(text)
	if rating < 1 || rating > 5 {
		return Review{}, &InvalidError{"rating must be between 1 and 5"}
	}
	if utf8.RuneCountInString(text) > 2000 {
		return Review{}, &InvalidError{"text must be at most 2000 characters"}
	}
	p, err := s.profiles.Profile(ctx, v, specialistID)
	if err != nil {
		return Review{}, err
	}
	lessonID, err := s.completedLesson(ctx, specialistID, v.UserID)
	if err != nil {
		return Review{}, err
	}
	var created bool
	err = s.db.QueryRow(ctx, `
		INSERT INTO reviews (specialist_id, student_id, lesson_id, rating, text) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (specialist_id, student_id) DO UPDATE
		SET rating = EXCLUDED.rating, text = EXCLUDED.text, lesson_id = EXCLUDED.lesson_id, updated_at = now()
		RETURNING xmax = 0`, specialistID, v.UserID, lessonID, rating, text).Scan(&created)
	if err != nil {
		return Review{}, fmt.Errorf("save review: %w", err)
	}
	if created {
		s.notifier.ReviewPosted(ctx, p.UserID, p.DisplayName, rating, text)
	}
	m, err := s.Mine(ctx, v, specialistID)
	if err != nil {
		return Review{}, err
	}
	return *m.Review, nil
}

// Delete removes the student's own review.
func (s *Service) Delete(ctx context.Context, studentID, specialistID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM reviews WHERE specialist_id = $1 AND student_id = $2`, specialistID, studentID)
	return err
}

// AdminList returns reviews with status, newest first.
func (s *Service) AdminList(ctx context.Context, status string) ([]AdminReview, error) {
	return s.adminQuery(ctx, `r.status = $1`, status)
}

func (s *Service) adminQuery(ctx context.Context, where string, arg any) ([]AdminReview, error) {
	rows, err := s.db.Query(ctx, `SELECT `+reviewColumns+`, r.status, sp.id, sp.display_name, sp.photo_url, r.student_id
		FROM reviews r JOIN users u ON u.telegram_id = r.student_id JOIN specialists sp ON sp.id = r.specialist_id
		WHERE `+where+` ORDER BY r.created_at DESC LIMIT 200`, arg)
	if err != nil {
		return nil, fmt.Errorf("select reviews: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AdminReview, error) {
		var a AdminReview
		var err error
		a.Review, err = s.scan(r, &a.Status, &a.Specialist.ID, &a.Specialist.DisplayName, &a.Specialist.PhotoURL, &a.StudentID)
		return a, err
	})
}

// SetStatus publishes or hides a review.
func (s *Service) SetStatus(ctx context.Context, id int64, status string) (AdminReview, error) {
	tag, err := s.db.Exec(ctx, `UPDATE reviews SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return AdminReview{}, fmt.Errorf("set review status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return AdminReview{}, ErrNotFound
	}
	list, err := s.adminQuery(ctx, `r.id = $1`, id)
	if err != nil || len(list) == 0 {
		return AdminReview{}, err
	}
	return list[0], nil
}

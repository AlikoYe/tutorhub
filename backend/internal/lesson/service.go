package lesson

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlikoYe/tutorhub/backend/internal/availability"
	"github.com/AlikoYe/tutorhub/backend/internal/lessonstatus"
	"github.com/AlikoYe/tutorhub/backend/internal/notify"
	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
	"github.com/AlikoYe/tutorhub/backend/internal/visibility"
)

// ListLimit caps lesson lists.
const ListLimit = 100

// Notifier tells people about lesson changes after they are committed. Implemented by bot.Notifier.
type Notifier interface {
	LessonRequested(ctx context.Context, l Lesson)
	LessonDecided(ctx context.Context, l Lesson)
	LessonCancelled(ctx context.Context, l Lesson)
	LessonExpired(ctx context.Context, l Lesson)
	LessonReminder(ctx context.Context, l Lesson)
}

// Events records funnel events (requests) for teacher statistics. Optional.
type Events interface {
	Record(ctx context.Context, specialistID int64, kind string, userID int64)
}

type Service struct {
	repo        *Repo
	schedules   *availability.Service
	specialists *specialist.Repo
	notifier    Notifier
	events      Events
	holdMinutes int
	loc         *time.Location
	log         *slog.Logger
	now         func() time.Time
}

func NewService(repo *Repo, schedules *availability.Service, specialists *specialist.Repo, notifier Notifier, holdMinutes int, loc *time.Location, log *slog.Logger) *Service {
	return &Service{repo: repo, schedules: schedules, specialists: specialists, notifier: notifier,
		holdMinutes: holdMinutes, loc: loc, log: log, now: time.Now}
}

// SetEvents plugs in statistics tracking (M3).
func (s *Service) SetEvents(e Events) { s.events = e }

// CanSeeAddress implements specialist.AddressAccess.
func (s *Service) CanSeeAddress(ctx context.Context, specialistID, userID int64) (bool, error) {
	return s.repo.HasConfirmed(ctx, specialistID, userID)
}

// CreateInput is a lesson request.
type CreateInput struct {
	StudentID    int64
	SpecialistID int64
	SubjectID    int64
	StartsAt     time.Time
	IsTrial      bool
	Note         string
}

// Create validates the request and inserts a pending lesson that holds the slot until the
// teacher answers (HOLD_MINUTES, never past the lesson start).
func (s *Service) Create(ctx context.Context, in CreateInput) (Lesson, error) {
	in.Note = strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(in.Note) > 500 {
		return Lesson{}, ErrNoteTooLong
	}
	b, err := s.repo.Bookable(ctx, in.SpecialistID, in.SubjectID)
	if errors.Is(err, ErrSpecialistGone) {
		return Lesson{}, ErrSpecialistGone
	}
	if err == nil && b.Status != visibility.Live {
		return Lesson{}, ErrSpecialistGone
	}
	if err != nil {
		return Lesson{}, err
	}
	if b.UserID == in.StudentID {
		return Lesson{}, ErrOwnProfile
	}
	price := b.PriceKZT
	if in.IsTrial {
		if b.TrialKind == "none" {
			return Lesson{}, ErrTrialUnavailable
		}
		had, err := s.repo.HadTrial(ctx, in.SpecialistID, in.StudentID)
		if err != nil {
			return Lesson{}, err
		}
		if had {
			return Lesson{}, ErrTrialUnavailable
		}
		price = 0
		if b.TrialKind == "paid" && b.TrialPrice != nil {
			price = *b.TrialPrice
		}
	}

	now := s.now()
	sched, err := s.schedules.Schedule(ctx, in.SpecialistID, in.StartsAt.In(s.loc).AddDate(0, 0, -1))
	if err != nil {
		return Lesson{}, err
	}
	if err := sched.Check(in.StartsAt, time.Duration(b.LessonMinutes)*time.Minute, now, s.loc); err != nil {
		return Lesson{}, err
	}

	expires := now.Add(time.Duration(s.holdMinutes) * time.Minute)
	if in.StartsAt.Before(expires) {
		expires = in.StartsAt
	}
	id, err := s.repo.Insert(ctx, insertRow{
		SpecialistID: in.SpecialistID, StudentID: in.StudentID, SubjectID: in.SubjectID,
		StartsAt: in.StartsAt, Minutes: b.LessonMinutes, ExpiresAt: expires,
		IsTrial: in.IsTrial, PriceKZT: price, Note: in.Note,
	})
	if err != nil {
		return Lesson{}, err
	}
	l, err := s.repo.Get(ctx, id)
	if err != nil {
		return Lesson{}, err
	}
	if s.events != nil {
		s.events.Record(ctx, in.SpecialistID, "request", in.StudentID)
	}
	s.notifier.LessonRequested(ctx, l)
	return l.ForStudent(s.now()), nil
}

// teacherLesson loads a lesson and checks actorID teaches it.
func (s *Service) teacherLesson(ctx context.Context, actorID, lessonID int64) (Lesson, error) {
	l, err := s.repo.Get(ctx, lessonID)
	if err != nil {
		return Lesson{}, err
	}
	if l.TeacherUserID != actorID {
		return Lesson{}, ErrForbidden
	}
	return l, nil
}

// Decide confirms or declines a pending request as the teacher; shared by the HTTP API and the bot.
func (s *Service) Decide(ctx context.Context, actorID, lessonID int64, confirm bool) (Lesson, error) {
	if _, err := s.teacherLesson(ctx, actorID, lessonID); err != nil {
		return Lesson{}, err
	}
	ok, err := s.repo.Decide(ctx, lessonID, confirm)
	if err != nil {
		return Lesson{}, err
	}
	if !ok {
		return Lesson{}, ErrNotPending
	}
	l, err := s.repo.Get(ctx, lessonID)
	if err != nil {
		return Lesson{}, err
	}
	s.notifier.LessonDecided(ctx, l)
	return l.ForTeacher(s.now()), nil
}

func cleanReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 500 {
		return "", ErrNoteTooLong
	}
	return reason, nil
}

// CancelAsStudent cancels the student's own upcoming lesson within the teacher's cancel rule.
func (s *Service) CancelAsStudent(ctx context.Context, studentID, lessonID int64, reason string) (Lesson, error) {
	reason, err := cleanReason(reason)
	if err != nil {
		return Lesson{}, err
	}
	l, err := s.repo.Get(ctx, lessonID)
	if err != nil {
		return Lesson{}, err
	}
	if l.StudentInfo.TelegramID != studentID {
		return Lesson{}, ErrNotFound
	}
	return s.cancel(ctx, lessonID, ByStudent, reason, func(l Lesson) Lesson { return l.ForStudent(s.now()) })
}

// CancelAsTeacher cancels any upcoming lesson of the teacher.
func (s *Service) CancelAsTeacher(ctx context.Context, actorID, lessonID int64, reason string) (Lesson, error) {
	reason, err := cleanReason(reason)
	if err != nil {
		return Lesson{}, err
	}
	if _, err := s.teacherLesson(ctx, actorID, lessonID); err != nil {
		return Lesson{}, err
	}
	return s.cancel(ctx, lessonID, ByTeacher, reason, func(l Lesson) Lesson { return l.ForTeacher(s.now()) })
}

func (s *Service) cancel(ctx context.Context, lessonID int64, by, reason string, view func(Lesson) Lesson) (Lesson, error) {
	ok, err := s.repo.Cancel(ctx, lessonID, by, reason)
	if err != nil {
		return Lesson{}, err
	}
	if !ok {
		return Lesson{}, ErrNotCancellable
	}
	l, err := s.repo.Get(ctx, lessonID)
	if err != nil {
		return Lesson{}, err
	}
	s.notifier.LessonCancelled(ctx, l)
	return view(l), nil
}

// Mine lists the student's lessons.
func (s *Service) Mine(ctx context.Context, studentID int64, scope string) ([]Lesson, error) {
	items, err := s.repo.ListForStudent(ctx, studentID, scope, ListLimit)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range items {
		items[i] = items[i].ForStudent(now)
	}
	return items, nil
}

// ForTeacher lists the teacher's lessons.
func (s *Service) ForTeacher(ctx context.Context, actorID int64, scope string) ([]Lesson, error) {
	id, err := s.specialists.IDByUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListForSpecialist(ctx, id, scope, ListLimit)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range items {
		items[i] = items[i].ForTeacher(now)
	}
	return items, nil
}

// Students lists the teacher's students.
func (s *Service) Students(ctx context.Context, actorID int64) ([]Student, error) {
	id, err := s.specialists.IDByUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return s.repo.Students(ctx, id)
}

// SetNote saves the teacher's private note about a student.
func (s *Service) SetNote(ctx context.Context, actorID, studentID int64, note string) error {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > 2000 {
		return ErrNoteTooLong
	}
	id, err := s.specialists.IDByUser(ctx, actorID)
	if err != nil {
		return err
	}
	ok, err := s.repo.SetNote(ctx, id, studentID, note)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// Jobs are the worker steps: expire unanswered requests, complete finished lessons, remind.
func (s *Service) Jobs(reminderBefore time.Duration) []notify.Job {
	return []notify.Job{
		notify.JobFunc{JobName: "lessons_expire", Fn: func(ctx context.Context) (int, error) {
			return s.claimAndNotify(ctx, s.notifier.LessonExpired, lessonstatus.ExpireAllOverdueSQL)
		}},
		notify.JobFunc{JobName: "lessons_complete", Fn: func(ctx context.Context) (int, error) {
			ids, err := s.repo.Claim(ctx, lessonstatus.CompleteFinishedSQL)
			return len(ids), err
		}},
		notify.JobFunc{JobName: "lessons_remind", Fn: func(ctx context.Context) (int, error) {
			return s.claimAndNotify(ctx, s.notifier.LessonReminder, lessonstatus.ClaimRemindersSQL, reminderBefore.Seconds())
		}},
	}
}

func (s *Service) claimAndNotify(ctx context.Context, send func(context.Context, Lesson), sql string, args ...any) (int, error) {
	ids, err := s.repo.Claim(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		l, err := s.repo.Get(ctx, id)
		if err != nil {
			s.log.WarnContext(ctx, "worker: load lesson", slog.Int64("lesson_id", id), slog.Any("err", err))
			continue
		}
		send(ctx, l)
	}
	return len(ids), nil
}

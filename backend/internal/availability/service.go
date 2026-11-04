package availability

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/AlikoYe/tutorhub/backend/internal/specialist"
)

var ErrUnknownSubject = errors.New("the specialist does not teach this subject")

// Profiles is what availability needs from the specialist module.
type Profiles interface {
	Profile(ctx context.Context, v specialist.Viewer, id int64) (specialist.Public, error)
	Repo() *specialist.Repo
}

type Service struct {
	repo     *Repo
	profiles Profiles
	loc      *time.Location
	now      func() time.Time
}

func NewService(repo *Repo, profiles Profiles, loc *time.Location) *Service {
	return &Service{repo: repo, profiles: profiles, loc: loc, now: time.Now}
}

// RuleJSON is a weekly window on the wire.
type RuleJSON struct {
	Weekday  int    `json:"weekday"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// Template is the teacher's own view of their availability.
type Template struct {
	Rules   []RuleJSON `json:"rules"`
	DaysOff []string   `json:"days_off"`
}

func (s *Service) today() time.Time {
	n := s.now().In(s.loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, s.loc)
}

func (s *Service) myID(ctx context.Context, userID int64) (int64, error) {
	return s.profiles.Repo().IDByUser(ctx, userID)
}

// Template returns the teacher's weekly windows and upcoming days off.
func (s *Service) Template(ctx context.Context, userID int64) (Template, error) {
	id, err := s.myID(ctx, userID)
	if err != nil {
		return Template{}, err
	}
	sched, err := s.repo.Schedule(ctx, id, s.today())
	if err != nil {
		return Template{}, err
	}
	t := Template{Rules: []RuleJSON{}, DaysOff: []string{}}
	for _, r := range sched.Rules {
		t.Rules = append(t.Rules, RuleJSON{Weekday: int(r.Weekday), StartsAt: FormatClock(r.Start), EndsAt: FormatClock(r.End)})
	}
	for d := range sched.Closed {
		t.DaysOff = append(t.DaysOff, d)
	}
	slices.Sort(t.DaysOff)
	return t, nil
}

// SetRules replaces the weekly template. Existing lessons are kept.
func (s *Service) SetRules(ctx context.Context, userID int64, in []RuleJSON) (Template, error) {
	id, err := s.myID(ctx, userID)
	if err != nil {
		return Template{}, err
	}
	rules := make([]Rule, 0, len(in))
	for _, r := range in {
		start, err := ParseClock(r.StartsAt)
		if err != nil {
			return Template{}, &InvalidError{err.Error()}
		}
		end, err := ParseClock(r.EndsAt)
		if err != nil {
			return Template{}, &InvalidError{err.Error()}
		}
		rules = append(rules, Rule{Weekday: time.Weekday(r.Weekday), Start: start, End: end})
	}
	if err := ValidateRules(rules); err != nil {
		return Template{}, &InvalidError{err.Error()}
	}
	if err := s.repo.ReplaceRules(ctx, id, rules); err != nil {
		return Template{}, err
	}
	return s.Template(ctx, userID)
}

// SetDayOff adds or removes a day off (today or later).
func (s *Service) SetDayOff(ctx context.Context, userID int64, day time.Time, closed bool) (Template, error) {
	id, err := s.myID(ctx, userID)
	if err != nil {
		return Template{}, err
	}
	d := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, s.loc)
	if d.Before(s.today()) || d.After(s.today().AddDate(1, 0, 0)) {
		return Template{}, &InvalidError{"day must be between today and one year ahead"}
	}
	if err := s.repo.SetDayOff(ctx, id, d, closed); err != nil {
		return Template{}, err
	}
	return s.Template(ctx, userID)
}

// Slots is the free time of a specialist for a subject.
type Slots struct {
	SpecialistID  int64 `json:"specialist_id"`
	LessonMinutes int   `json:"lesson_minutes"`
	Days          []Day `json:"days"`
}

// LessonLength is the lesson length of subjectID for p (0 = the default length).
func LessonLength(p specialist.Public, subjectID int64) (int, error) {
	if subjectID == 0 {
		return p.LessonMinutes, nil
	}
	for _, o := range p.Subjects {
		if o.SubjectID == subjectID {
			return o.LessonMinutes, nil
		}
	}
	return 0, ErrUnknownSubject
}

// FreeSlots lists free start times for days local dates from `from` (default today).
func (s *Service) FreeSlots(ctx context.Context, v specialist.Viewer, specialistID, subjectID int64, from *time.Time, days int) (Slots, error) {
	p, err := s.profiles.Profile(ctx, v, specialistID)
	if err != nil {
		return Slots{}, err
	}
	minutes, err := LessonLength(p, subjectID)
	if err != nil {
		return Slots{}, err
	}
	start := s.today()
	if from != nil {
		start = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, s.loc)
	}
	if start.Before(s.today()) {
		start = s.today()
	}
	last := s.today().AddDate(0, 0, MaxDaysAhead+1)
	if end := start.AddDate(0, 0, days); end.After(last) {
		days = int(last.Sub(start).Hours()/24 + 0.5)
	}
	if days < 0 {
		days = 0
	}
	sched, err := s.repo.Schedule(ctx, specialistID, start)
	if err != nil {
		return Slots{}, err
	}
	busy, err := s.repo.Busy(ctx, specialistID, start, start.AddDate(0, 0, days+1))
	if err != nil {
		return Slots{}, err
	}
	return Slots{
		SpecialistID:  specialistID,
		LessonMinutes: minutes,
		Days:          sched.Slots(start, days, time.Duration(minutes)*time.Minute, s.now(), busy, s.loc),
	}, nil
}

// Schedule exposes the raw schedule for lesson validation.
func (s *Service) Schedule(ctx context.Context, specialistID int64, from time.Time) (Schedule, error) {
	return s.repo.Schedule(ctx, specialistID, from)
}

// InvalidError is a 400-class input error.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

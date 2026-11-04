// Package lesson handles lesson requests: a student requests a free slot, the teacher confirms
// or declines (in the app or the bot), either side cancels within the rules, the worker
// expires unanswered requests, completes finished lessons and sends reminders.
package lesson

import (
	"errors"
	"time"

	"github.com/AlikoYe/tutorhub/backend/internal/lessonstatus"
)

var (
	ErrNotFound          = errors.New("lesson not found")
	ErrForbidden         = errors.New("only the teacher of this lesson can do this")
	ErrSpecialistGone    = errors.New("specialist not found")
	ErrOwnProfile        = errors.New("you cannot request a lesson from yourself")
	ErrSubjectNotOffered = errors.New("the teacher does not teach this subject")
	ErrTrialUnavailable  = errors.New("a trial lesson is not available with this teacher")
	ErrSlotTaken         = errors.New("this time is already taken")
	ErrStudentBusy       = errors.New("you already have a lesson at that time")
	ErrNotPending        = errors.New("lesson request is no longer pending")
	ErrNotCancellable    = errors.New("lesson can no longer be cancelled")
	ErrNoteTooLong       = errors.New("note must be at most 500 characters")
)

// Values of lessons.cancelled_by.
const (
	ByStudent = "student"
	ByTeacher = "teacher"
)

type SubjectRef struct {
	ID     int64  `json:"id"`
	Slug   string `json:"slug"`
	NameRU string `json:"name_ru"`
	NameKK string `json:"name_kk"`
	NameEN string `json:"name_en"`
}

type SpecialistRef struct {
	ID          int64   `json:"id"`
	DisplayName string  `json:"display_name"`
	PhotoURL    *string `json:"photo_url"`
	Address     *string `json:"address"` // only once the lesson is confirmed
}

type Person struct {
	TelegramID int64   `json:"telegram_id"`
	Name       string  `json:"name"`
	Username   *string `json:"username"`
}

// Lesson is one lesson request as a student or teacher sees it.
type Lesson struct {
	ID           int64         `json:"id"`
	Status       string        `json:"status"`
	StartsAt     time.Time     `json:"starts_at"`
	EndsAt       time.Time     `json:"ends_at"`
	ExpiresAt    *time.Time    `json:"expires_at"`
	IsTrial      bool          `json:"is_trial"`
	PriceKZT     int           `json:"price_kzt"`
	Note         string        `json:"note"`
	CancelledBy  *string       `json:"cancelled_by"`
	CancelReason string        `json:"cancel_reason"`
	CreatedAt    time.Time     `json:"created_at"`
	CanCancel    bool          `json:"can_cancel"`
	Subject      SubjectRef    `json:"subject"`
	Specialist   SpecialistRef `json:"specialist"`
	Student      *Person       `json:"student,omitempty"` // teacher views only

	StudentInfo   Person `json:"-"`
	TeacherUserID int64  `json:"-"`
	CancelHours   int    `json:"-"`
	address       string
}

// ForStudent prepares l for the student: address after confirmation, the cancel rule.
func (l Lesson) ForStudent(now time.Time) Lesson {
	l.Student = nil
	l.Specialist.Address = nil
	if (l.Status == lessonstatus.Confirmed || l.Status == lessonstatus.Completed) && l.address != "" {
		a := l.address
		l.Specialist.Address = &a
	}
	l.CanCancel = l.StartsAt.After(now) && (l.Status == lessonstatus.Pending ||
		(l.Status == lessonstatus.Confirmed && l.StartsAt.Sub(now) >= time.Duration(l.CancelHours)*time.Hour))
	return l
}

// ForTeacher prepares l for the teacher: student details; the teacher may cancel any upcoming lesson.
func (l Lesson) ForTeacher(now time.Time) Lesson {
	p := l.StudentInfo
	l.Student = &p
	l.Specialist.Address = nil
	l.CanCancel = l.StartsAt.After(now) && (l.Status == lessonstatus.Pending || l.Status == lessonstatus.Confirmed)
	return l
}

// Student is one row of the teacher's students list.
type Student struct {
	Person
	Lessons      int        `json:"lessons"`        // confirmed or completed
	Requests     int        `json:"requests"`       // every request ever made
	NextLessonAt *time.Time `json:"next_lesson_at"` // next confirmed lesson
	LastLessonAt *time.Time `json:"last_lesson_at"` // last completed lesson
	Note         string     `json:"note"`
}

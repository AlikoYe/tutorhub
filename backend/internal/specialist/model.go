// Package specialist owns teacher/consultant profiles: the owner's editor, submission and
// admin verification, photos, and the catalogue (search + public profile).
package specialist

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlikoYe/tutorhub/backend/internal/geo"
)

var (
	ErrNotFound      = errors.New("specialist not found")
	ErrNoProfile     = errors.New("you have no specialist profile yet")
	ErrInvalidStatus = errors.New("this action is not allowed in the profile's current status")
	ErrBlocked       = errors.New("this profile is blocked")
	ErrUnknownSubj   = errors.New("unknown subject")
	ErrBadPhoto      = errors.New("photo must be a JPEG, PNG or WebP image up to 5 MB")
)

// IncompleteError lists what a profile still needs before it can be submitted.
type IncompleteError struct{ Missing []string }

func (e *IncompleteError) Error() string {
	return "profile is incomplete: " + strings.Join(e.Missing, ", ")
}

// InvalidError is a validation failure of an edit request (400).
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &InvalidError{Msg: fmt.Sprintf(format, args...)}
}

// Offer is one subject a specialist teaches, with levels and price per lesson.
type Offer struct {
	SubjectID int64    `json:"subject_id"`
	Slug      string   `json:"slug"`
	NameRU    string   `json:"name_ru"`
	NameKK    string   `json:"name_kk"`
	NameEN    string   `json:"name_en"`
	Levels    []string `json:"levels"`
	PriceKZT  int      `json:"price_kzt"`
	// LessonMinutes is the lesson length for this subject (the specialist's default unless overridden).
	LessonMinutes int `json:"lesson_minutes"`
}

// Card is a specialist in search results.
type Card struct {
	ID              int64    `json:"id"`
	Kind            string   `json:"kind"`
	DisplayName     string   `json:"display_name"`
	PhotoURL        *string  `json:"photo_url"`
	Verified        bool     `json:"verified"`
	Featured        bool     `json:"featured"`
	City            string   `json:"city"`
	District        string   `json:"district"`
	Formats         []string `json:"formats"`
	Languages       []string `json:"languages"`
	ExperienceYears int      `json:"experience_years"`
	TrialKind       string   `json:"trial_kind"`
	TrialPrice      *int     `json:"trial_price"`
	MinPriceKZT     *int     `json:"min_price_kzt"`
	Rating          *float64 `json:"rating"` // average of published reviews, one decimal
	ReviewsCount    int      `json:"reviews_count"`
	Subjects        []Offer  `json:"subjects"`
}

// Public is the profile page anyone with the link sees.
type Public struct {
	Card
	Bio           string  `json:"bio"`
	Education     string  `json:"education"`
	LessonMinutes int     `json:"lesson_minutes"`
	CancelHours   int     `json:"cancel_hours"` // students may cancel a confirmed lesson up to this many hours before
	Address       *string `json:"address"`      // only for the owner, admins (and, later, confirmed students)
	Listed        bool    `json:"listed"`       // shown in search right now
	Favourite     bool    `json:"favourite"`    // the viewer saved it

	UserID int64 `json:"-"` // the owner's Telegram id
}

// Subscription is the plan that makes a specialist visible.
type Subscription struct {
	ID       int64     `json:"id"`
	Plan     string    `json:"plan"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Note     string    `json:"note"`
}

// Owner identifies the Telegram user behind a profile (admin views).
type Owner struct {
	TelegramID int64   `json:"telegram_id"`
	Name       string  `json:"name"`
	Username   *string `json:"username"`
}

// Private is the full profile for its owner and admins.
type Private struct {
	Public
	Status          string        `json:"status"`
	RejectionReason *string       `json:"rejection_reason"`
	SubmittedAt     *time.Time    `json:"submitted_at"`
	ApprovedAt      *time.Time    `json:"approved_at"`
	Lat             *float64      `json:"lat"`
	Lng             *float64      `json:"lng"`
	Subscription    *Subscription `json:"subscription"` // running now, or the latest one
	PersonalLink    *string       `json:"personal_link"`
	Owner           Owner         `json:"owner"`

	VerificationStatus string `json:"verification_status"` // none, requested, verified, rejected
	VerificationNote   string `json:"verification_note"`
	BlockReason        string `json:"block_reason"`
}

// Summary is the user's own profile in /me.
type Summary struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	DisplayName string `json:"display_name"`
	Listed      bool   `json:"listed"`
}

// OfferInput is one subject in an edit request.
type OfferInput struct {
	SubjectID     int64    `json:"subject_id"`
	Levels        []string `json:"levels"`
	PriceKZT      int      `json:"price_kzt"`
	LessonMinutes *int     `json:"lesson_minutes"` // nil = the profile default
}

// Input is the editable part of a profile (PUT /my/specialist replaces all of it).
type Input struct {
	Kind            string       `json:"kind"`
	DisplayName     string       `json:"display_name"`
	Bio             string       `json:"bio"`
	Education       string       `json:"education"`
	ExperienceYears int          `json:"experience_years"`
	City            string       `json:"city"`
	District        string       `json:"district"`
	Address         string       `json:"address"`
	Lat             *float64     `json:"lat"`
	Lng             *float64     `json:"lng"`
	Formats         []string     `json:"formats"`
	Languages       []string     `json:"languages"`
	TrialKind       string       `json:"trial_kind"`
	TrialPrice      *int         `json:"trial_price"`
	LessonMinutes   int          `json:"lesson_minutes"`
	CancelHours     *int         `json:"cancel_hours"` // nil = 12
	Subjects        []OfferInput `json:"subjects"`
}

const (
	maxName      = 80
	maxBio       = 2000
	maxEducation = 1000
	maxAddress   = 200
	maxSubjects  = 20
	minBio       = 30
)

// Normalize trims text, fills defaults and validates every field. Completeness for
// submission is checked separately (Missing).
func (in *Input) Normalize() error {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.Bio = strings.TrimSpace(in.Bio)
	in.Education = strings.TrimSpace(in.Education)
	in.Address = strings.TrimSpace(in.Address)
	in.City = strings.TrimSpace(in.City)
	if in.Kind == "" {
		in.Kind = "teacher"
	}
	if in.City == "" {
		in.City = geo.DefaultCity
	}
	if in.TrialKind == "" {
		in.TrialKind = "none"
	}
	if in.LessonMinutes == 0 {
		in.LessonMinutes = 60
	}
	if in.CancelHours == nil {
		h := 12
		in.CancelHours = &h
	}
	if in.Formats == nil {
		in.Formats = []string{}
	}
	if in.Languages == nil {
		in.Languages = []string{}
	}

	switch {
	case in.Kind != "teacher" && in.Kind != "consultant":
		return invalid("kind must be teacher or consultant")
	case utf8.RuneCountInString(in.DisplayName) > maxName:
		return invalid("display_name must be at most %d characters", maxName)
	case utf8.RuneCountInString(in.Bio) > maxBio:
		return invalid("bio must be at most %d characters", maxBio)
	case utf8.RuneCountInString(in.Education) > maxEducation:
		return invalid("education must be at most %d characters", maxEducation)
	case utf8.RuneCountInString(in.Address) > maxAddress:
		return invalid("address must be at most %d characters", maxAddress)
	case in.ExperienceYears < 0 || in.ExperienceYears > 70:
		return invalid("experience_years must be between 0 and 70")
	case func() bool { _, ok := geo.FindCity(in.City); return !ok }():
		return invalid("unknown city %q", in.City)
	case !geo.ValidDistrict(in.City, in.District):
		return invalid("unknown district %q for %s", in.District, in.City)
	case (in.Lat == nil) != (in.Lng == nil):
		return invalid("lat and lng must be given together")
	case in.Lat != nil && (*in.Lat < -90 || *in.Lat > 90 || *in.Lng < -180 || *in.Lng > 180):
		return invalid("lat/lng are out of range")
	case !geo.AllIn(in.Formats, geo.Formats):
		return invalid("formats must be distinct values of online, teacher_place, student_home")
	case !geo.AllIn(in.Languages, geo.Languages):
		return invalid("languages must be distinct values of kk, ru, en")
	case in.TrialKind != "none" && in.TrialKind != "free" && in.TrialKind != "paid":
		return invalid("trial_kind must be none, free or paid")
	case in.TrialKind == "paid" && (in.TrialPrice == nil || *in.TrialPrice < 0 || *in.TrialPrice > 1000000):
		return invalid("trial_price (0..1000000) is required for a paid trial")
	case in.LessonMinutes < 30 || in.LessonMinutes > 240 || in.LessonMinutes%15 != 0:
		return invalid("lesson_minutes must be a multiple of 15 between 30 and 240")
	case *in.CancelHours < 0 || *in.CancelHours > 72:
		return invalid("cancel_hours must be between 0 and 72")
	case len(in.Subjects) > maxSubjects:
		return invalid("at most %d subjects", maxSubjects)
	}
	if in.TrialKind != "paid" {
		in.TrialPrice = nil
	}

	seen := map[int64]bool{}
	for i := range in.Subjects {
		o := &in.Subjects[i]
		if o.Levels == nil {
			o.Levels = []string{}
		}
		switch {
		case o.SubjectID <= 0:
			return invalid("subjects[%d].subject_id is required", i)
		case seen[o.SubjectID]:
			return invalid("subject %d is listed twice", o.SubjectID)
		case !geo.AllIn(o.Levels, geo.Levels):
			return invalid("subjects[%d].levels must be distinct values of %s", i, strings.Join(geo.Levels, ", "))
		case o.PriceKZT < 0 || o.PriceKZT > 1000000:
			return invalid("subjects[%d].price_kzt must be between 0 and 1000000", i)
		case o.LessonMinutes != nil && (*o.LessonMinutes < 30 || *o.LessonMinutes > 240 || *o.LessonMinutes%15 != 0):
			return invalid("subjects[%d].lesson_minutes must be a multiple of 15 between 30 and 240", i)
		}
		seen[o.SubjectID] = true
	}
	return nil
}

// Missing lists what p still needs before submission.
func Missing(p Private) []string {
	var m []string
	if p.DisplayName == "" {
		m = append(m, "display_name")
	}
	if utf8.RuneCountInString(p.Bio) < minBio {
		m = append(m, fmt.Sprintf("bio (at least %d characters)", minBio))
	}
	if len(p.Subjects) == 0 {
		m = append(m, "subjects")
	}
	for _, o := range p.Subjects {
		if len(o.Levels) == 0 {
			m = append(m, "levels for "+o.Slug)
		}
	}
	if len(p.Formats) == 0 {
		m = append(m, "formats")
	}
	if len(p.Languages) == 0 {
		m = append(m, "languages")
	}
	offline := false
	for _, f := range p.Formats {
		offline = offline || f != "online"
	}
	if offline && p.District == "" {
		m = append(m, "district")
	}
	return m
}

// SpecialistRef is a short reference used by other modules' responses.
type SpecialistRef struct {
	ID          int64   `json:"id"`
	DisplayName string  `json:"display_name"`
	PhotoURL    *string `json:"photo_url"`
}

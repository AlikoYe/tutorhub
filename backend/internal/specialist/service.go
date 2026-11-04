package specialist

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlikoYe/tutorhub/backend/internal/visibility"
)

// Notifier tells people about profile changes after they are committed. Implemented by bot.Notifier.
type Notifier interface {
	SpecialistSubmitted(ctx context.Context, adminIDs []int64, p Private)
	SpecialistApproved(ctx context.Context, p Private)
	SpecialistRejected(ctx context.Context, p Private, reason string)
}

// Viewer is who is looking at a profile.
type Viewer struct {
	UserID  int64
	IsAdmin bool
}

// AddressAccess reports whether a viewer may see a specialist's exact address
// (a confirmed lesson with them). Optional; nil means only owner and admins.
type AddressAccess interface {
	CanSeeAddress(ctx context.Context, specialistID, userID int64) (bool, error)
}

type Service struct {
	repo       *Repo
	photos     PhotoStore
	notifier   Notifier
	personal   func(id int64) string
	address    AddressAccess
	trialDays  int
	loc        *time.Location
	log        *slog.Logger
	now        func() time.Time
	queueLimit int
}

// Options configures NewService.
type Options struct {
	Photos       PhotoStore
	Notifier     Notifier
	PersonalLink func(id int64) string // "" when no bot username is configured
	TrialDays    int
	Location     *time.Location
	Log          *slog.Logger
}

func NewService(repo *Repo, o Options) *Service {
	return &Service{repo: repo, photos: o.Photos, notifier: o.Notifier, personal: o.PersonalLink,
		trialDays: o.TrialDays, loc: o.Location, log: o.Log, now: time.Now, queueLimit: 200}
}

// SetAddressAccess plugs in the lessons module (M2) for address visibility.
func (s *Service) SetAddressAccess(a AddressAccess) { s.address = a }

// Repo exposes the repository to sibling modules.
func (s *Service) Repo() *Repo { return s.repo }

func (s *Service) withLink(p Private) Private {
	if s.personal != nil {
		if l := s.personal(p.ID); l != "" {
			p.PersonalLink = &l
		}
	}
	return p
}

// MySpecialist implements auth.SpecialistLookup for /me.
func (s *Service) MySpecialist(ctx context.Context, userID int64) (any, error) {
	sum, err := s.repo.Summary(ctx, userID)
	if err != nil || sum == nil {
		return nil, err
	}
	return sum, nil
}

func (s *Service) My(ctx context.Context, userID int64) (Private, error) {
	p, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return Private{}, err
	}
	return s.withLink(p), nil
}

// Save creates or updates the user's profile. Status does not change.
func (s *Service) Save(ctx context.Context, userID int64, in Input) (Private, error) {
	if err := in.Normalize(); err != nil {
		return Private{}, err
	}
	if _, err := s.repo.Upsert(ctx, userID, in); err != nil {
		return Private{}, err
	}
	return s.My(ctx, userID)
}

// Submit sends a complete draft to the admins for verification.
func (s *Service) Submit(ctx context.Context, userID int64) (Private, error) {
	p, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return Private{}, err
	}
	if p.Status != visibility.Draft {
		return Private{}, ErrInvalidStatus
	}
	if missing := Missing(p); len(missing) > 0 {
		return Private{}, &IncompleteError{Missing: missing}
	}
	if err := s.repo.Submit(ctx, p.ID); err != nil {
		return Private{}, err
	}
	p, err = s.My(ctx, userID)
	if err != nil {
		return Private{}, err
	}
	admins, err := s.repo.AdminIDs(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "list admins for notification", slog.Any("err", err))
	}
	s.notifier.SpecialistSubmitted(ctx, admins, p)
	return p, nil
}

// SetHidden lets a live teacher pause their listing and come back.
func (s *Service) SetHidden(ctx context.Context, userID int64, hidden bool) (Private, error) {
	p, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return Private{}, err
	}
	if hidden {
		err = s.repo.Hide(ctx, p.ID)
	} else {
		err = s.repo.Unhide(ctx, p.ID)
	}
	if err != nil {
		return Private{}, err
	}
	return s.My(ctx, userID)
}

// UploadPhoto replaces the user's profile photo.
func (s *Service) UploadPhoto(ctx context.Context, userID int64, data []byte) (Private, error) {
	p, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return Private{}, err
	}
	if p.Status == visibility.Blocked {
		return Private{}, ErrBlocked
	}
	url, err := s.photos.Save(p.ID, data)
	if err != nil {
		return Private{}, err
	}
	old, err := s.repo.SetPhoto(ctx, p.ID, url)
	if err != nil {
		s.photos.Remove(url)
		return Private{}, err
	}
	if old != nil {
		s.photos.Remove(*old)
	}
	return s.My(ctx, userID)
}

// Profile returns the public profile. Others see it only while it is live (listed or not);
// the owner and admins always do and also see the address.
func (s *Service) Profile(ctx context.Context, v Viewer, id int64) (Public, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return Public{}, err
	}
	own := v.IsAdmin || p.UserID == v.UserID
	if !own && p.Status != visibility.Live {
		return Public{}, ErrNotFound
	}
	pub := p.Public
	if v.UserID != 0 {
		if pub.Favourite, err = s.repo.IsFavourite(ctx, v.UserID, id); err != nil {
			return Public{}, err
		}
	}
	if !own {
		ok := false
		if s.address != nil && v.UserID != 0 {
			if ok, err = s.address.CanSeeAddress(ctx, id, v.UserID); err != nil {
				return Public{}, err
			}
		}
		if !ok || pub.Address == nil || *pub.Address == "" {
			pub.Address = nil
		}
	} else if pub.Address != nil && *pub.Address == "" {
		pub.Address = nil
	}
	return pub, nil
}

// SetFavourite adds or removes a favourite.
func (s *Service) SetFavourite(ctx context.Context, userID, specialistID int64, on bool) error {
	ok, err := s.repo.SetFavourite(ctx, userID, specialistID, on)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Favourites(ctx context.Context, userID int64) ([]Card, error) {
	return s.repo.Favourites(ctx, userID)
}

// ContactLink is where the "contact" button leads.
type ContactLink struct {
	TelegramURL string `json:"telegram_url"`
}

// Contact returns a link to the specialist's Telegram chat; the caller records the lead.
func (s *Service) Contact(ctx context.Context, v Viewer, id int64) (ContactLink, Public, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return ContactLink{}, Public{}, err
	}
	if p.Status != visibility.Live && !v.IsAdmin && p.UserID != v.UserID {
		return ContactLink{}, Public{}, ErrNotFound
	}
	link := ContactLink{TelegramURL: fmt.Sprintf("tg://user?id=%d", p.UserID)}
	if p.Owner.Username != nil && *p.Owner.Username != "" {
		link.TelegramURL = "https://t.me/" + *p.Owner.Username
	}
	return link, p.Public, nil
}

// Search runs a catalogue search.
func (s *Service) Search(ctx context.Context, f Filter) (Page, error) {
	return s.repo.Search(ctx, f)
}

// Queue lists profiles in status for admins (default pending).
func (s *Service) Queue(ctx context.Context, status string) ([]Private, error) {
	items, err := s.repo.ListByStatus(ctx, status, s.queueLimit)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i] = s.withLink(items[i])
	}
	return items, nil
}

func (s *Service) AdminGet(ctx context.Context, id int64) (Private, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return Private{}, err
	}
	return s.withLink(p), nil
}

// ApproveInput chooses the plan a newly approved profile starts with.
type ApproveInput struct {
	Plan   string     // trial (default) or founding
	EndsOn *time.Time // last day of the plan (date in the app zone); required for founding
}

// Approve makes a pending profile live and starts its trial or founding plan.
func (s *Service) Approve(ctx context.Context, adminID, id int64, in ApproveInput) (Private, error) {
	if in.Plan == "" {
		in.Plan = visibility.PlanTrial
	}
	if in.Plan != visibility.PlanTrial && in.Plan != visibility.PlanFounding {
		return Private{}, invalid("plan must be trial or founding")
	}
	var endsAt time.Time
	switch {
	case in.EndsOn != nil:
		endsAt = EndOfDay(*in.EndsOn, s.loc)
	case in.Plan == visibility.PlanFounding:
		return Private{}, invalid("ends_on is required for the founding plan")
	default:
		endsAt = EndOfDay(s.now().In(s.loc).AddDate(0, 0, s.trialDays), s.loc)
	}
	if !endsAt.After(s.now()) {
		return Private{}, invalid("ends_on must be today or later")
	}
	if _, err := s.repo.Approve(ctx, id, adminID, in.Plan, endsAt); err != nil {
		return Private{}, err
	}
	p, err := s.AdminGet(ctx, id)
	if err != nil {
		return Private{}, err
	}
	s.notifier.SpecialistApproved(ctx, p)
	return p, nil
}

// Reject sends a pending profile back to draft with a reason the teacher sees.
func (s *Service) Reject(ctx context.Context, id int64, reason string) (Private, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		return Private{}, invalid("reason is required (at most 500 characters)")
	}
	if err := s.repo.Reject(ctx, id, reason); err != nil {
		return Private{}, err
	}
	p, err := s.AdminGet(ctx, id)
	if err != nil {
		return Private{}, err
	}
	s.notifier.SpecialistRejected(ctx, p, reason)
	return p, nil
}

// EndOfDay is the first instant after calendar day d in loc (plans end at local midnight).
func EndOfDay(d time.Time, loc *time.Location) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, loc)
}

// IsValidation reports whether err is a 400-class validation error.
func IsValidation(err error) bool {
	var ie *InvalidError
	return errors.As(err, &ie)
}

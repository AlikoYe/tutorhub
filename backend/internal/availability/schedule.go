// Package availability holds a specialist's weekly template and days off, and computes free
// lesson slots from them.
package availability

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// Step is the grid lesson starts snap to.
	Step = 30 * time.Minute
	// Lead is how far ahead a lesson must start to be requested.
	Lead = time.Hour
	// MaxDaysAhead bounds slot queries and lesson requests.
	MaxDaysAhead = 30
)

var (
	ErrOutside     = errors.New("this time is outside the teacher's working hours")
	ErrTooSoon     = errors.New("a lesson must start at least 1 hour from now")
	ErrTooFarAhead = errors.New("lessons can be requested at most 30 days ahead")
)

// Rule is one weekly working window; Start/End are minutes since midnight (End may be 1440).
type Rule struct {
	Weekday    time.Weekday
	Start, End int
}

// Schedule is everything needed to compute slots.
type Schedule struct {
	Rules  []Rule
	Closed map[string]bool // YYYY-MM-DD -> day off
}

// Interval is a half-open time range [Start, End).
type Interval struct {
	Start, End time.Time
}

func (i Interval) overlaps(o Interval) bool { return i.Start.Before(o.End) && o.Start.Before(i.End) }

// Slot is a free start time.
type Slot struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

// Day groups the free slots of one local date.
type Day struct {
	Date  string `json:"date"`
	Slots []Slot `json:"slots"`
}

// candidates lists every grid start of length that fits a window on local date d.
func (s Schedule) candidates(d time.Time, length time.Duration, loc *time.Location) []Interval {
	if s.Closed[d.Format(time.DateOnly)] {
		return nil
	}
	midnight := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	var out []Interval
	for _, r := range s.Rules {
		if r.Weekday != d.Weekday() {
			continue
		}
		end := midnight.Add(time.Duration(r.End) * time.Minute)
		for st := midnight.Add(time.Duration(r.Start) * time.Minute); !st.Add(length).After(end); st = st.Add(Step) {
			out = append(out, Interval{st, st.Add(length)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// Slots returns free slots of length for days local dates starting at from, skipping times
// before now+Lead and any slot overlapping busy.
func (s Schedule) Slots(from time.Time, days int, length time.Duration, now time.Time, busy []Interval, loc *time.Location) []Day {
	out := make([]Day, 0, days)
	from = from.In(loc)
	for i := 0; i < days; i++ {
		d := time.Date(from.Year(), from.Month(), from.Day()+i, 12, 0, 0, 0, loc)
		day := Day{Date: d.Format(time.DateOnly), Slots: []Slot{}}
		seen := map[time.Time]bool{}
		for _, c := range s.candidates(d, length, loc) {
			if c.Start.Before(now.Add(Lead)) || seen[c.Start] {
				continue
			}
			free := true
			for _, b := range busy {
				if c.overlaps(b) {
					free = false
					break
				}
			}
			if free {
				seen[c.Start] = true
				day.Slots = append(day.Slots, Slot{StartsAt: c.Start, EndsAt: c.End})
			}
		}
		out = append(out, day)
	}
	return out
}

// Check reports whether a lesson of length may start at start: on the grid, inside a working
// window, not on a day off, at least Lead from now and at most MaxDaysAhead days ahead.
func (s Schedule) Check(start time.Time, length time.Duration, now time.Time, loc *time.Location) error {
	start = start.In(loc)
	if start.Before(now.Add(Lead)) {
		return ErrTooSoon
	}
	today := now.In(loc)
	limit := time.Date(today.Year(), today.Month(), today.Day()+MaxDaysAhead+1, 0, 0, 0, 0, loc)
	if !start.Before(limit) {
		return ErrTooFarAhead
	}
	for _, c := range s.candidates(start, length, loc) {
		if c.Start.Equal(start) {
			return nil
		}
	}
	return ErrOutside
}

// ParseClock parses "HH:MM" (00:00..24:00) into minutes since midnight.
func ParseClock(v string) (int, error) {
	h, m, ok := strings.Cut(v, ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || len(h) != 2 || len(m) != 2 || err1 != nil || err2 != nil || hh < 0 || hh > 24 || mm < 0 || mm > 59 || (hh == 24 && mm != 0) {
		return 0, fmt.Errorf("time %q must be HH:MM between 00:00 and 24:00", v)
	}
	return hh*60 + mm, nil
}

// FormatClock formats minutes since midnight as HH:MM.
func FormatClock(min int) string {
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

// ValidateRules checks each window is non-empty, on the 15-minute grid and that windows of a
// weekday do not overlap.
func ValidateRules(rules []Rule) error {
	if len(rules) > 50 {
		return errors.New("at most 50 working windows")
	}
	byDay := map[time.Weekday][]Rule{}
	for _, r := range rules {
		switch {
		case r.Weekday < 0 || r.Weekday > 6:
			return errors.New("weekday must be 0 (Sunday) to 6 (Saturday)")
		case r.Start >= r.End:
			return fmt.Errorf("window %s-%s ends before it starts", FormatClock(r.Start), FormatClock(r.End))
		case r.Start%15 != 0 || r.End%15 != 0:
			return errors.New("window times must be multiples of 15 minutes")
		}
		byDay[r.Weekday] = append(byDay[r.Weekday], r)
	}
	for _, list := range byDay {
		sort.Slice(list, func(i, j int) bool { return list[i].Start < list[j].Start })
		for i := 1; i < len(list); i++ {
			if list[i].Start < list[i-1].End {
				return errors.New("working windows of the same day overlap")
			}
		}
	}
	return nil
}

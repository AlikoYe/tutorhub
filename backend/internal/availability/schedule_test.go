package availability

import (
	"testing"
	"time"
)

var almaty, _ = time.LoadLocation("Asia/Almaty")

// Monday 2026-11-09 and Wednesday 2026-11-11.
func at(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, almaty) }

var sched = Schedule{
	Rules: []Rule{
		{Weekday: time.Monday, Start: 16 * 60, End: 18 * 60},
		{Weekday: time.Wednesday, Start: 22 * 60, End: 24 * 60},
	},
	Closed: map[string]bool{"2026-11-16": true},
}

func TestSlots(t *testing.T) {
	now := at(8, 9, 0) // Sunday
	busy := []Interval{{at(9, 16, 30), at(9, 17, 30)}}
	days := sched.Slots(at(9, 0, 0), 9, time.Hour, now, busy, almaty)
	if len(days) != 9 || days[0].Date != "2026-11-09" {
		t.Fatalf("days %+v", days)
	}
	// Monday 16:00-18:00 with 60-minute lessons: 16:00, 16:30 and 17:00 all overlap 16:30-17:30.
	if got := len(days[0].Slots); got != 0 {
		t.Errorf("monday slots %v, want none (every start overlaps 16:30-17:30)", days[0].Slots)
	}
	// Wednesday 22:00-24:00: 22:00, 22:30, 23:00.
	wed := days[2]
	if len(wed.Slots) != 3 || !wed.Slots[2].StartsAt.Equal(at(11, 23, 0)) || !wed.Slots[2].EndsAt.Equal(at(12, 0, 0)) {
		t.Errorf("wednesday %+v", wed)
	}
	// Next Monday (16th) is a day off.
	if len(days[7].Slots) != 0 {
		t.Errorf("day off has slots %+v", days[7])
	}
}

func TestSlotsSkipsTooSoon(t *testing.T) {
	days := sched.Slots(at(9, 0, 0), 1, time.Hour, at(9, 15, 30), nil, almaty)
	if len(days[0].Slots) != 2 || !days[0].Slots[0].StartsAt.Equal(at(9, 16, 30)) {
		t.Fatalf("slots %+v, want 16:30 and 17:00", days[0].Slots)
	}
}

func TestCheck(t *testing.T) {
	now := at(8, 9, 0)
	cases := []struct {
		start time.Time
		want  error
	}{
		{at(9, 16, 0), nil},
		{at(9, 17, 0), nil},
		{at(9, 17, 30), ErrOutside}, // would end at 18:30
		{at(9, 16, 15), ErrOutside}, // off grid
		{at(10, 16, 0), ErrOutside}, // Tuesday
		{at(16, 16, 0), ErrOutside}, // day off
		{at(8, 9, 30), ErrTooSoon},
		{at(9, 16, 0).AddDate(0, 0, 35), ErrTooFarAhead},
		{at(9, 16, 0).UTC(), nil}, // any offset is fine
	}
	for _, c := range cases {
		if err := sched.Check(c.start, time.Hour, now, almaty); err != c.want {
			t.Errorf("Check(%v) = %v, want %v", c.start, err, c.want)
		}
	}
}

func TestClock(t *testing.T) {
	for in, want := range map[string]int{"00:00": 0, "09:30": 570, "24:00": 1440} {
		if got, err := ParseClock(in); err != nil || got != want {
			t.Errorf("ParseClock(%q) = %d, %v", in, got, err)
		}
	}
	for _, bad := range []string{"9:30", "24:30", "25:00", "10:60", "ab:cd", ""} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("ParseClock(%q) should fail", bad)
		}
	}
	if FormatClock(1440) != "24:00" || FormatClock(75) != "01:15" {
		t.Error("FormatClock")
	}
}

func TestValidateRules(t *testing.T) {
	ok := []Rule{{1, 600, 720}, {1, 720, 780}, {2, 600, 720}}
	if err := ValidateRules(ok); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]Rule{
		{{1, 600, 600}},
		{{1, 600, 720}, {1, 700, 800}},
		{{7, 600, 720}},
		{{1, 601, 720}},
	} {
		if ValidateRules(bad) == nil {
			t.Errorf("ValidateRules(%v) should fail", bad)
		}
	}
}

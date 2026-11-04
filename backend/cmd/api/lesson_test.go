package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

type lessonJSON struct {
	ID          int64      `json:"id"`
	Status      string     `json:"status"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      time.Time  `json:"ends_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	IsTrial     bool       `json:"is_trial"`
	PriceKZT    int        `json:"price_kzt"`
	CanCancel   bool       `json:"can_cancel"`
	CancelledBy *string    `json:"cancelled_by"`
	Specialist  struct {
		ID      int64   `json:"id"`
		Address *string `json:"address"`
	} `json:"specialist"`
	Student *struct {
		TelegramID int64 `json:"telegram_id"`
	} `json:"student"`
}

// tomorrow returns hh:mm tomorrow in Almaty.
func (e *env) tomorrow(h, m int) time.Time {
	d := time.Now().In(e.loc).AddDate(0, 0, 1)
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, e.loc)
}

// allWeek opens every day 00:00-24:00 for the teacher.
func (e *env) allWeek(teacher int64) {
	e.t.Helper()
	var rules []map[string]any
	for wd := 0; wd < 7; wd++ {
		rules = append(rules, map[string]any{"weekday": wd, "starts_at": "00:00", "ends_at": "24:00"})
	}
	e.mustCall(http.MethodPut, "/api/teacher/availability/rules", teacher, map[string]any{"rules": rules}, http.StatusOK, nil)
}

// teacherWithSchedule makes a live teacher (math, 5000 ₸, 60 min, free trial) open all week.
func (e *env) teacherWithSchedule(user int64, mutate ...func(map[string]any)) int64 {
	e.t.Helper()
	id := e.liveSpecialist(user, e.profileInput(fmt.Sprintf("Teacher %d", user), "math", 5000, mutate...))
	e.allWeek(user)
	return id
}

func (e *env) request(student, specialistID int64, start time.Time, extra ...map[string]any) response {
	e.t.Helper()
	body := map[string]any{"specialist_id": specialistID, "subject_id": e.subjectID("math"), "starts_at": start.Format(time.RFC3339)}
	for _, x := range extra {
		for k, v := range x {
			body[k] = v
		}
	}
	return e.call(http.MethodPost, "/api/lessons", student, body)
}

func (e *env) mustRequest(student, specialistID int64, start time.Time) lessonJSON {
	e.t.Helper()
	r := e.request(student, specialistID, start)
	if r.code != http.StatusCreated {
		e.t.Fatalf("request lesson: %d %s", r.code, r.body)
	}
	var l lessonJSON
	if err := jsonUnmarshal(r.body, &l); err != nil {
		e.t.Fatal(err)
	}
	return l
}

// insertLesson writes a lesson directly: during = [start, start+1h).
func (e *env) insertLesson(specialistID, student int64, start time.Time, status string, expiresAt *time.Time) int64 {
	e.t.Helper()
	e.exec(`INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING`, student)
	return e.id(`INSERT INTO lessons (specialist_id, student_id, subject_id, during, status, expires_at)
		VALUES ($1, $2, (SELECT id FROM subjects WHERE slug = 'math'),
		        tstzrange($3::timestamptz, $3::timestamptz + interval '1 hour', '[)'), $4, $5) RETURNING id`,
		specialistID, student, start, status, expiresAt)
}

func TestAvailabilityTemplate(t *testing.T) {
	e := newEnv(t)
	e.wantError(e.call(http.MethodGet, "/api/teacher/availability", 1, nil), http.StatusNotFound, "not_found")
	e.liveSpecialist(10, e.profileInput("T", "math", 5000))

	rules := []map[string]any{
		{"weekday": 1, "starts_at": "16:00", "ends_at": "20:00"},
		{"weekday": 3, "starts_at": "09:00", "ends_at": "12:00"},
		{"weekday": 3, "starts_at": "22:00", "ends_at": "24:00"},
	}
	var tpl struct {
		Rules []struct {
			Weekday  int    `json:"weekday"`
			StartsAt string `json:"starts_at"`
			EndsAt   string `json:"ends_at"`
		} `json:"rules"`
		DaysOff []string `json:"days_off"`
	}
	e.mustCall(http.MethodPut, "/api/teacher/availability/rules", 10, map[string]any{"rules": rules}, http.StatusOK, &tpl)
	if len(tpl.Rules) != 3 || tpl.Rules[2].EndsAt != "24:00" || tpl.Rules[0].StartsAt != "16:00" {
		t.Fatalf("rules %+v", tpl.Rules)
	}
	for _, bad := range []any{
		[]map[string]any{{"weekday": 1, "starts_at": "16:00", "ends_at": "15:00"}},
		[]map[string]any{{"weekday": 1, "starts_at": "16:00", "ends_at": "18:00"}, {"weekday": 1, "starts_at": "17:00", "ends_at": "19:00"}},
		[]map[string]any{{"weekday": 1, "starts_at": "16:10", "ends_at": "18:00"}},
		[]map[string]any{{"weekday": 7, "starts_at": "16:00", "ends_at": "18:00"}},
		nil,
	} {
		e.wantError(e.call(http.MethodPut, "/api/teacher/availability/rules", 10, map[string]any{"rules": bad}), http.StatusBadRequest, "bad_request")
	}

	day := e.tomorrow(0, 0).Format(time.DateOnly)
	e.mustCall(http.MethodPut, "/api/teacher/availability/days-off/"+day, 10, nil, http.StatusOK, &tpl)
	if len(tpl.DaysOff) != 1 || tpl.DaysOff[0] != day {
		t.Fatalf("days off %v", tpl.DaysOff)
	}
	e.mustCall(http.MethodDelete, "/api/teacher/availability/days-off/"+day, 10, nil, http.StatusOK, &tpl)
	if len(tpl.DaysOff) != 0 {
		t.Fatalf("days off after delete %v", tpl.DaysOff)
	}
	e.wantError(e.call(http.MethodPut, "/api/teacher/availability/days-off/2020-01-01", 10, nil), http.StatusBadRequest, "bad_request")
	e.wantError(e.call(http.MethodPut, "/api/teacher/availability/days-off/tomorrow", 10, nil), http.StatusBadRequest, "bad_request")
}

type slotsJSON struct {
	LessonMinutes int `json:"lesson_minutes"`
	Days          []struct {
		Date  string `json:"date"`
		Slots []struct {
			StartsAt time.Time `json:"starts_at"`
		} `json:"slots"`
	} `json:"days"`
}

func (s slotsJSON) starts(i int) []string {
	out := []string{}
	for _, sl := range s.Days[i].Slots {
		out = append(out, sl.StartsAt.Format("15:04"))
	}
	return out
}

func TestSlots(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(20, e.profileInput("T", "math", 5000, func(in map[string]any) {
		in["subjects"] = []map[string]any{
			{"subject_id": e.subjectID("math"), "levels": []string{"high"}, "price_kzt": 5000},
			{"subject_id": e.subjectID("physics"), "levels": []string{"high"}, "price_kzt": 6000, "lesson_minutes": 90},
		}
	}))
	var rules []map[string]any
	for wd := 0; wd < 7; wd++ {
		rules = append(rules, map[string]any{"weekday": wd, "starts_at": "09:00", "ends_at": "13:00"})
	}
	e.mustCall(http.MethodPut, "/api/teacher/availability/rules", 20, map[string]any{"rules": rules}, http.StatusOK, nil)
	from := e.tomorrow(0, 0).Format(time.DateOnly)
	path := fmt.Sprintf("/api/specialists/%d/slots?from=%s&days=2", id, from)

	var s slotsJSON
	e.mustCall(http.MethodGet, path, 30, nil, http.StatusOK, &s)
	if s.LessonMinutes != 60 || len(s.Days) != 2 || s.Days[0].Date != from || fmt.Sprint(s.starts(0)) != "[09:00 09:30 10:00 10:30 11:00 11:30 12:00]" {
		t.Fatalf("slots %+v %v", s, s.starts(0))
	}

	e.mustRequest(31, id, e.tomorrow(10, 0))
	e.mustCall(http.MethodGet, path, 30, nil, http.StatusOK, &s)
	if got := fmt.Sprint(s.starts(0)); got != "[09:00 11:00 11:30 12:00]" {
		t.Fatalf("after a request: %s", got)
	}

	// Per-subject length: 90 minutes.
	e.mustCall(http.MethodGet, path+fmt.Sprintf("&subject_id=%d", e.subjectID("physics")), 30, nil, http.StatusOK, &s)
	if s.LessonMinutes != 90 || fmt.Sprint(s.starts(1)) != "[09:00 09:30 10:00 10:30 11:00 11:30]" {
		t.Fatalf("physics %d %v", s.LessonMinutes, s.starts(1))
	}
	e.wantError(e.call(http.MethodGet, path+fmt.Sprintf("&subject_id=%d", e.subjectID("chess")), 30, nil), http.StatusBadRequest, "bad_request")

	// Day off.
	e.mustCall(http.MethodPut, "/api/teacher/availability/days-off/"+from, 20, nil, http.StatusOK, nil)
	e.mustCall(http.MethodGet, path, 30, nil, http.StatusOK, &s)
	if len(s.Days[0].Slots) != 0 || len(s.Days[1].Slots) == 0 {
		t.Fatalf("day off: %+v", s.Days)
	}

	// Past the 30-day horizon days are cut.
	far := time.Now().In(e.loc).AddDate(0, 0, 29).Format(time.DateOnly)
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d/slots?from=%s&days=7", id, far), 30, nil, http.StatusOK, &s)
	if len(s.Days) != 2 {
		t.Fatalf("horizon: %d days", len(s.Days))
	}

	e.mustCall(http.MethodPost, "/api/my/specialist/hide", 20, nil, http.StatusOK, nil)
	e.wantError(e.call(http.MethodGet, path, 30, nil), http.StatusNotFound, "not_found")
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/specialists/%d/slots?days=40", id), 20, nil), http.StatusBadRequest, "bad_request")
}

func TestLessonLifecycle(t *testing.T) {
	e := newEnv(t)
	const teacher, student = 40, 41
	id := e.teacherWithSchedule(teacher)

	l := e.mustRequest(student, id, e.tomorrow(15, 0))
	if l.Status != "pending" || l.ExpiresAt == nil || l.PriceKZT != 5000 || l.Specialist.Address != nil || l.Student != nil || !l.CanCancel {
		t.Fatalf("created %+v", l)
	}
	if d := time.Until(*l.ExpiresAt); d < 5*time.Hour || d > 7*time.Hour {
		t.Errorf("hold %v, want ~6h", d)
	}
	e.notifier.Wait()
	e.sender.mu.Lock()
	last := e.sender.sent[len(e.sender.sent)-1]
	e.sender.mu.Unlock()
	kb, ok := last.ReplyMarkup.(*models.InlineKeyboardMarkup)
	if last.ChatID != int64(teacher) || !ok || kb.InlineKeyboard[0][0].CallbackData != fmt.Sprintf("ls:c:%d", l.ID) {
		t.Fatalf("teacher message %+v", last)
	}

	var list []lessonJSON
	e.mustCall(http.MethodGet, "/api/teacher/lessons?scope=requests", teacher, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Student == nil || list[0].Student.TelegramID != student {
		t.Fatalf("teacher requests %+v", list)
	}
	e.mustCall(http.MethodGet, "/api/lessons/my", student, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].ID != l.ID {
		t.Fatalf("my lessons %+v", list)
	}

	// Only the teacher decides.
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/confirm", l.ID), student, nil), http.StatusForbidden, "forbidden")
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/confirm", l.ID), teacher, nil, http.StatusOK, &l)
	if l.Status != "confirmed" {
		t.Fatalf("status %s", l.Status)
	}
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/decline", l.ID), teacher, nil), http.StatusConflict, "not_pending")
	e.notifier.Wait()
	if e.sender.countContaining(student, "Урок подтверждён") != 1 || e.sender.countContaining(student, "Тимирязева") != 1 {
		t.Errorf("student messages %v", e.sender.messagesTo(student))
	}

	// The address opens to the student after confirmation.
	e.mustCall(http.MethodGet, "/api/lessons/my", student, nil, http.StatusOK, &list)
	if list[0].Specialist.Address == nil {
		t.Error("student does not see the address of a confirmed lesson")
	}
	var p specialistJSON
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d", id), student, nil, http.StatusOK, &p)
	if p.Address == nil {
		t.Error("profile hides the address from a confirmed student")
	}

	// The teacher cancels; the student is told.
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/cancel", l.ID), teacher, map[string]any{"reason": "Заболела"}, http.StatusOK, &l)
	if l.Status != "cancelled" || l.CancelledBy == nil || *l.CancelledBy != "teacher" {
		t.Fatalf("cancelled %+v", l)
	}
	e.notifier.Wait()
	if e.sender.countContaining(student, "Заболела") != 1 {
		t.Errorf("student not told: %v", e.sender.messagesTo(student))
	}
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/lessons/%d/cancel", l.ID), student, nil), http.StatusConflict, "not_cancellable")
	e.mustCall(http.MethodGet, "/api/lessons/my?scope=past", student, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Status != "cancelled" {
		t.Fatalf("past %+v", list)
	}

	// The slot is free again.
	e.mustRequest(42, id, e.tomorrow(15, 0))
}

func TestStudentCancelRule(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(50, func(in map[string]any) { in["cancel_hours"] = 24 })
	soon := time.Now().Add(5 * time.Hour).Truncate(time.Hour)
	confirmed := e.insertLesson(id, 51, soon, "confirmed", nil)
	later := e.insertLesson(id, 51, soon.Add(48*time.Hour), "confirmed", nil)
	exp := time.Now().Add(time.Hour)
	pending := e.insertLesson(id, 51, soon.Add(2*time.Hour), "pending", &exp)

	var list []lessonJSON
	e.mustCall(http.MethodGet, "/api/lessons/my", 51, nil, http.StatusOK, &list)
	can := map[int64]bool{}
	for _, l := range list {
		can[l.ID] = l.CanCancel
	}
	if can[confirmed] || !can[later] || !can[pending] {
		t.Fatalf("can_cancel %v", can)
	}
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/lessons/%d/cancel", confirmed), 51, nil), http.StatusConflict, "not_cancellable")
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/lessons/%d/cancel", later), 51, map[string]any{"reason": "Уезжаем"}, http.StatusOK, nil)
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/lessons/%d/cancel", pending), 51, nil, http.StatusOK, nil)
	e.notifier.Wait()
	if e.sender.countContaining(50, "Уезжаем") != 1 {
		t.Errorf("teacher not told: %v", e.sender.messagesTo(50))
	}
	// Someone else's lesson reads as not found; the teacher may still cancel the close one.
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/lessons/%d/cancel", confirmed), 52, nil), http.StatusNotFound, "not_found")
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/cancel", confirmed), 50, nil, http.StatusOK, nil)
}

func TestLessonValidation(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(60)
	noTrial := e.teacherWithSchedule(61, func(in map[string]any) { in["trial_kind"] = "none" })
	start := e.tomorrow(12, 0)

	e.wantError(e.request(60, id, start), http.StatusUnprocessableEntity, "own_profile")
	e.wantError(e.request(70, id, start, map[string]any{"subject_id": e.subjectID("chess")}), http.StatusUnprocessableEntity, "subject_not_offered")
	e.wantError(e.request(70, id, e.tomorrow(12, 15)), http.StatusUnprocessableEntity, "outside_availability")
	e.wantError(e.request(70, id, start.AddDate(0, 0, 40)), http.StatusUnprocessableEntity, "outside_availability")
	e.wantError(e.request(70, id, time.Now().Add(20*time.Minute)), http.StatusUnprocessableEntity, "too_soon")
	e.wantError(e.request(70, 999999, start), http.StatusNotFound, "not_found")
	e.wantError(e.request(70, id, start, map[string]any{"note": strings.Repeat("я", 501)}), http.StatusBadRequest, "bad_request")
	e.wantError(e.request(70, noTrial, start, map[string]any{"is_trial": true}), http.StatusUnprocessableEntity, "trial_unavailable")
	e.wantError(e.call(http.MethodPost, "/api/lessons", 70, map[string]any{"subject_id": 1}), http.StatusBadRequest, "bad_request")

	// One trial per student per teacher; a free trial costs 0.
	r := e.request(70, id, start, map[string]any{"is_trial": true})
	var l lessonJSON
	if r.code != http.StatusCreated || jsonUnmarshal(r.body, &l) != nil || !l.IsTrial || l.PriceKZT != 0 {
		t.Fatalf("trial: %d %s", r.code, r.body)
	}
	e.wantError(e.request(70, id, start.Add(2*time.Hour), map[string]any{"is_trial": true}), http.StatusUnprocessableEntity, "trial_unavailable")

	// A teacher outside working hours.
	e.mustCall(http.MethodPut, "/api/teacher/availability/rules", 60, map[string]any{"rules": []any{}}, http.StatusOK, nil)
	e.wantError(e.request(71, id, start.Add(4*time.Hour)), http.StatusUnprocessableEntity, "outside_availability")

	// A teacher whose subscription ended can still be booked through their link (status live).
	e.exec(`UPDATE subscriptions SET ends_at = now() - interval '1 minute', starts_at = now() - interval '2 days' WHERE specialist_id = $1`, noTrial)
	e.mustRequest(72, noTrial, start)
	e.mustCall(http.MethodPost, "/api/my/specialist/hide", 61, nil, http.StatusOK, nil)
	e.wantError(e.request(73, noTrial, start.Add(2*time.Hour)), http.StatusNotFound, "not_found")
}

// The database makes double booking impossible: 20 students racing for one slot get exactly
// one pending lesson and 19 clean 409s (no deadlocks, no 500s).
func TestConcurrentRequestsForOneSlot(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(80)
	for round := 0; round < 3; round++ {
		start := e.tomorrow(9+3*round, 0)
		res := make([]response, 20)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				res[i] = e.request(int64(1000+round*100+i), id, start)
			}(i)
		}
		wg.Wait()
		created, taken := 0, 0
		for _, r := range res {
			switch {
			case r.code == http.StatusCreated:
				created++
			case r.code == http.StatusConflict && r.errorCode() == "slot_taken":
				taken++
			default:
				t.Errorf("round %d: unexpected %d %s", round, r.code, r.body)
			}
		}
		if created != 1 || taken != 19 {
			t.Fatalf("round %d: %d created, %d slot_taken; want 1 and 19", round, created, taken)
		}
	}
}

func TestConcurrentRequestsByOneStudent(t *testing.T) {
	e := newEnv(t)
	var teachers []int64
	for i := range 4 {
		teachers = append(teachers, e.teacherWithSchedule(int64(90+i)))
	}
	start := e.tomorrow(14, 0)
	res := make([]response, len(teachers))
	var wg sync.WaitGroup
	for i, id := range teachers {
		wg.Add(1)
		go func(i int, id int64) {
			defer wg.Done()
			res[i] = e.request(99, id, start)
		}(i, id)
	}
	wg.Wait()
	created, busy := 0, 0
	for _, r := range res {
		switch {
		case r.code == http.StatusCreated:
			created++
		case r.code == http.StatusConflict && r.errorCode() == "student_busy":
			busy++
		default:
			t.Errorf("unexpected %d %s", r.code, r.body)
		}
	}
	if created != 1 || busy != 3 {
		t.Fatalf("%d created, %d student_busy; want 1 and 3", created, busy)
	}
	// Back-to-back is fine.
	e.mustRequest(99, teachers[0], start.Add(time.Hour))
}

// An overdue request no longer holds the slot: the next request expires it and succeeds.
func TestExpireBeforeInsert(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(100)
	start := e.tomorrow(16, 0)
	past := time.Now().Add(-time.Minute)
	old := e.insertLesson(id, 101, start, "pending", &past)

	var list []lessonJSON
	e.mustCall(http.MethodGet, "/api/lessons/my?scope=past", 101, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Status != "expired" || list[0].ExpiresAt != nil {
		t.Fatalf("overdue reads as %+v", list)
	}
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/confirm", old), 100, nil), http.StatusConflict, "not_pending")
	e.mustRequest(102, id, start)
	var status string
	if err := e.db.QueryRow(context.Background(), `SELECT status FROM lessons WHERE id = $1`, old).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("old row status %q %v", status, err)
	}
}

func TestWorkerJobs(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(110)
	past := time.Now().Add(-time.Minute)
	e.insertLesson(id, 111, e.tomorrow(10, 0), "pending", &past)
	soon := time.Now().Add(90 * time.Minute)
	e.insertLesson(id, 112, soon, "confirmed", nil)
	finished := e.insertLesson(id, 113, time.Now().Add(-3*time.Hour), "confirmed", nil)
	e.insertLesson(id, 114, time.Now().Add(5*time.Hour), "confirmed", nil) // too far for a reminder

	w := e.app.worker()
	res := w.RunOnce(context.Background())
	if res["lessons_expire"] != 1 || res["lessons_complete"] != 1 || res["lessons_remind"] != 1 {
		t.Fatalf("first run %v", res)
	}
	res = w.RunOnce(context.Background())
	if res["lessons_expire"]+res["lessons_complete"]+res["lessons_remind"] != 0 {
		t.Fatalf("second run must be a no-op: %v", res)
	}
	e.notifier.Wait()
	if e.sender.countContaining(111, "истекла") != 1 {
		t.Errorf("expired: %v", e.sender.messagesTo(111))
	}
	if e.sender.countContaining(112, "Напоминание") != 1 || e.sender.countContaining(110, "Напоминание") != 1 {
		t.Errorf("reminders: student %v teacher %v", e.sender.messagesTo(112), e.sender.messagesTo(110))
	}
	var status string
	if err := e.db.QueryRow(context.Background(), `SELECT status FROM lessons WHERE id = $1`, finished).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("finished lesson status %q", status)
	}
}

func TestBotConfirmDecline(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(120)
	a := e.mustRequest(121, id, e.tomorrow(10, 0))
	b := e.mustRequest(122, id, e.tomorrow(12, 0))
	press := func(from int64, data string) {
		e.bot.HandleUpdate(context.Background(), &models.Update{CallbackQuery: &models.CallbackQuery{
			ID: "q", From: models.User{ID: from}, Data: data,
			Message: models.MaybeInaccessibleMessage{Message: &models.Message{ID: 7, Chat: models.Chat{ID: from}, Text: "🆕 Заявка"}},
		}})
	}
	press(121, fmt.Sprintf("ls:c:%d", a.ID)) // the student cannot confirm
	press(120, fmt.Sprintf("ls:c:%d", a.ID))
	press(120, fmt.Sprintf("ls:d:%d", b.ID))
	press(120, fmt.Sprintf("ls:d:%d", b.ID)) // already decided

	var list []lessonJSON
	e.mustCall(http.MethodGet, "/api/teacher/lessons?scope=upcoming", 120, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].ID != a.ID || list[0].Status != "confirmed" {
		t.Fatalf("upcoming %+v", list)
	}
	e.sender.mu.Lock()
	defer e.sender.mu.Unlock()
	if len(e.sender.answered) != 4 || !e.sender.answered[0].ShowAlert || e.sender.answered[1].ShowAlert || !e.sender.answered[3].ShowAlert {
		t.Fatalf("answers %+v", e.sender.answered)
	}
	if len(e.sender.edited) != 3 || !strings.Contains(e.sender.edited[0].Text, "Подтверждено") || !strings.Contains(e.sender.edited[2].Text, "больше не активна") {
		t.Fatalf("edits %+v", e.sender.edited)
	}
}

func TestStudentsAndNotes(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(130)
	l := e.mustRequest(131, id, e.tomorrow(10, 0))
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/confirm", l.ID), 130, nil, http.StatusOK, nil)
	e.mustRequest(132, id, e.tomorrow(12, 0))
	e.insertLesson(id, 131, time.Now().Add(-48*time.Hour), "completed", nil)

	e.mustCall(http.MethodPut, "/api/teacher/students/131/note", 130, map[string]any{"note": "Цель — 130 баллов"}, http.StatusNoContent, nil)
	e.wantError(e.call(http.MethodPut, "/api/teacher/students/999/note", 130, map[string]any{"note": "x"}), http.StatusNotFound, "not_found")

	var list []struct {
		TelegramID   int64      `json:"telegram_id"`
		Lessons      int        `json:"lessons"`
		Requests     int        `json:"requests"`
		NextLessonAt *time.Time `json:"next_lesson_at"`
		LastLessonAt *time.Time `json:"last_lesson_at"`
		Note         string     `json:"note"`
	}
	e.mustCall(http.MethodGet, "/api/teacher/students", 130, nil, http.StatusOK, &list)
	if len(list) != 2 || list[0].TelegramID != 131 || list[0].Lessons != 2 || list[0].Requests != 2 ||
		list[0].NextLessonAt == nil || list[0].LastLessonAt == nil || list[0].Note != "Цель — 130 баллов" {
		t.Fatalf("students %+v", list)
	}
	if list[1].TelegramID != 132 || list[1].Lessons != 0 || list[1].NextLessonAt != nil {
		t.Fatalf("second student %+v", list[1])
	}
	e.wantError(e.call(http.MethodGet, "/api/teacher/students", 131, nil), http.StatusNotFound, "not_found")
}

func TestFreeThisWeekFilterAndRanking(t *testing.T) {
	e := newEnv(t)
	busy := e.liveSpecialist(140, e.profileInput("Busy", "math", 5000)) // no availability at all
	free := e.teacherWithSchedule(141)
	full := e.liveSpecialist(142, e.profileInput("Full", "math", 5000))
	// Full: one 2-hour window tomorrow, fully booked.
	wd := int(e.tomorrow(0, 0).Weekday())
	e.mustCall(http.MethodPut, "/api/teacher/availability/rules", 142, map[string]any{"rules": []map[string]any{
		{"weekday": wd, "starts_at": "10:00", "ends_at": "12:00"},
	}}, http.StatusOK, nil)
	e.insertLesson(full, 143, e.tomorrow(10, 0), "confirmed", nil)
	e.insertLesson(full, 144, e.tomorrow(11, 0), "confirmed", nil)

	if got := ids(e.search(1, "free_this_week=true")); fmt.Sprint(got) != fmt.Sprint([]int64{free}) {
		t.Fatalf("free_this_week = %v, want [%d]", got, free)
	}
	// Free time ranks above newer approvals.
	if got := ids(e.search(1, "")); got[0] != free {
		t.Fatalf("ranking %v, want %d first (busy %d, full %d)", got, free, busy, full)
	}
}

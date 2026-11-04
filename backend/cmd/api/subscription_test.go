package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

type subJSON struct {
	ID       int64     `json:"id"`
	Plan     string    `json:"plan"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

type stateJSON struct {
	Listed         bool       `json:"listed"`
	Current        *subJSON   `json:"current"`
	CoverageEndsAt *time.Time `json:"coverage_ends_at"`
	History        []subJSON  `json:"history"`
}

func TestPlans(t *testing.T) {
	e := newEnv(t, func(c *configT) { c.PriceBasicKZT, c.PriceFeaturedKZT, c.PaymentInstructions = 2990, 7990, "Kaspi" })
	var p struct {
		Plans []struct {
			Plan     string `json:"plan"`
			PriceKZT int    `json:"price_kzt"`
		} `json:"plans"`
		PaymentInstructions string `json:"payment_instructions"`
	}
	e.mustCall(http.MethodGet, "/api/plans", 1, nil, http.StatusOK, &p)
	if len(p.Plans) != 2 || p.Plans[0].PriceKZT != 2990 || p.Plans[1].Plan != "featured" || p.Plans[1].PriceKZT != 7990 || p.PaymentInstructions != "Kaspi" {
		t.Fatalf("plans %+v", p)
	}
}

func TestActivateAndExtend(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(200, e.profileInput("Sub", "math", 5000))
	path := fmt.Sprintf("/api/admin/specialists/%d/subscriptions", id)

	var st stateJSON
	e.mustCall(http.MethodGet, "/api/teacher/subscription", 200, nil, http.StatusOK, &st)
	if !st.Listed || st.Current == nil || st.Current.Plan != "trial" || len(st.History) != 1 {
		t.Fatalf("initial %+v", st)
	}
	trialEnd := st.Current.EndsAt

	// Paid basic for 1 month starts now (no running basic) and does not end the trial.
	e.mustCall(http.MethodPost, path, adminID, map[string]any{"plan": "basic", "months": 1, "note": "Kaspi #123"}, http.StatusOK, &st)
	if len(st.History) != 2 {
		t.Fatalf("history %+v", st.History)
	}
	var basic subJSON
	for _, x := range st.History {
		if x.Plan == "basic" {
			basic = x
		}
	}
	if basic.Plan != "basic" || time.Since(basic.StartsAt) > time.Minute {
		t.Fatalf("basic %+v", basic)
	}
	// Extending basic starts where the first one ends.
	e.mustCall(http.MethodPost, path, adminID, map[string]any{"plan": "basic", "months": 2}, http.StatusOK, &st)
	if st.History[0].Plan != "basic" || !st.History[0].StartsAt.Equal(basic.EndsAt) || !st.History[0].EndsAt.Equal(basic.EndsAt.In(e.loc).AddDate(0, 2, 0)) {
		t.Fatalf("extension %+v, first ended %v", st.History[0], basic.EndsAt)
	}
	if st.CoverageEndsAt == nil || !st.CoverageEndsAt.Equal(st.History[0].EndsAt) || !st.CoverageEndsAt.After(trialEnd) {
		t.Fatalf("coverage %v", st.CoverageEndsAt)
	}
	// Featured wins as the current plan.
	e.mustCall(http.MethodPost, path, adminID, map[string]any{"plan": "featured", "ends_on": time.Now().In(e.loc).AddDate(0, 0, 10).Format(time.DateOnly)}, http.StatusOK, &st)
	if st.Current == nil || st.Current.Plan != "featured" {
		t.Fatalf("current %+v", st.Current)
	}
	e.notifier.Wait()
	if n := e.sender.countContaining(200, "Подписка"); n != 3 {
		t.Errorf("teacher got %d subscription messages: %v", n, e.sender.messagesTo(200))
	}

	for _, bad := range []map[string]any{
		{"plan": "basic"},
		{"plan": "basic", "months": 1, "ends_on": "2030-01-01"},
		{"plan": "gold", "months": 1},
		{"plan": "basic", "months": 25},
		{"plan": "basic", "ends_on": "2020-01-01"},
		{"plan": "basic", "ends_on": "soon"},
	} {
		e.wantError(e.call(http.MethodPost, path, adminID, bad), http.StatusBadRequest, "bad_request")
	}
	e.wantError(e.call(http.MethodPost, path, 200, map[string]any{"plan": "basic", "months": 1}), http.StatusForbidden, "forbidden")
	e.wantError(e.call(http.MethodPost, "/api/admin/specialists/99999/subscriptions", adminID, map[string]any{"plan": "basic", "months": 1}), http.StatusNotFound, "not_found")
	e.wantError(e.call(http.MethodGet, "/api/teacher/subscription", 201, nil), http.StatusNotFound, "not_found")
}

func TestEndSubscription(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(210, e.profileInput("End", "math", 5000))
	var st stateJSON
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/admin/specialists/%d/subscriptions", id), adminID, nil, http.StatusOK, &st)
	trial := st.Current.ID
	future := e.id(`INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at) VALUES ($1, 'basic', now() + interval '40 days', now() + interval '70 days') RETURNING id`, id)

	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/subscriptions/%d/end", future), adminID, nil, http.StatusOK, &st)
	if len(st.History) != 1 {
		t.Fatalf("future not deleted: %+v", st.History)
	}
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/subscriptions/%d/end", trial), adminID, nil, http.StatusOK, &st)
	if st.Listed || st.Current != nil || st.CoverageEndsAt != nil {
		t.Fatalf("after end %+v", st)
	}
	// Ending an ended one is harmless; unknown is 404.
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/subscriptions/%d/end", trial), adminID, nil, http.StatusOK, nil)
	e.wantError(e.call(http.MethodPost, "/api/admin/subscriptions/99999/end", adminID, nil), http.StatusNotFound, "not_found")
	if got := ids(e.search(1, "")); len(got) != 0 {
		t.Fatalf("still listed %v", got)
	}
}

func TestFoundingPeriod(t *testing.T) {
	e := newEnv(t)
	e.user(adminID, true)
	var a, b specialistJSON
	for i, p := range []*specialistJSON{&a, &b} {
		u := int64(220 + i)
		e.mustCall(http.MethodPut, "/api/my/specialist", u, e.profileInput("F", "math", 5000), http.StatusOK, p)
		e.mustCall(http.MethodPost, "/api/my/specialist/submit", u, nil, http.StatusOK, nil)
		e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/approve", p.ID), adminID, map[string]any{"plan": "founding", "ends_on": "2030-01-31"}, http.StatusOK, nil)
	}
	e.liveSpecialist(222, e.profileInput("Trial", "math", 5000))

	var res struct {
		Updated int `json:"updated"`
	}
	e.mustCall(http.MethodPost, "/api/admin/founding-period", adminID, map[string]any{"ends_on": "2031-06-30"}, http.StatusOK, &res)
	if res.Updated != 2 {
		t.Fatalf("updated %d, want 2", res.Updated)
	}
	var st stateJSON
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/admin/specialists/%d/subscriptions", a.ID), adminID, nil, http.StatusOK, &st)
	if want := time.Date(2031, 7, 1, 0, 0, 0, 0, e.loc); !st.Current.EndsAt.Equal(want) {
		t.Fatalf("founding ends %v, want %v", st.Current.EndsAt, want)
	}
	e.wantError(e.call(http.MethodPost, "/api/admin/founding-period", adminID, map[string]any{}), http.StatusBadRequest, "bad_request")
	e.wantError(e.call(http.MethodPost, "/api/admin/founding-period", 220, map[string]any{"ends_on": "2031-06-30"}), http.StatusForbidden, "forbidden")
}

func TestSubscriptionWorker(t *testing.T) {
	e := newEnv(t)
	mk := func(user int64, startsAgo, endsIn string) int64 {
		id := e.liveSpecialist(user, e.profileInput(fmt.Sprint(user), "math", 5000))
		e.exec(`DELETE FROM subscriptions WHERE specialist_id = $1`, id)
		e.exec(`INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at) VALUES ($1, 'basic', now() - $2::interval, now() + $3::interval)`, id, startsAgo, endsIn)
		return id
	}
	mk(230, "20 days", "5 days")   // 7-day reminder
	mk(231, "20 days", "12 hours") // 1-day reminder
	mk(232, "30 days", "-1 hour")  // just expired
	mk(233, "60 days", "-10 days") // long gone: silent
	renewed := mk(234, "20 days", "5 days")
	e.exec(`INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at) VALUES ($1, 'basic', now() + interval '5 days', now() + interval '35 days')`, renewed)
	mk(235, "1 day", "20 days") // nothing yet

	w := e.app.worker()
	res := w.RunOnce(context.Background())
	if res["subscriptions_remind_7d"] != 1 || res["subscriptions_remind_1d"] != 1 || res["subscriptions_expired"] != 1 {
		t.Fatalf("first run %v", res)
	}
	res = w.RunOnce(context.Background())
	if res["subscriptions_remind_7d"]+res["subscriptions_remind_1d"]+res["subscriptions_expired"] != 0 {
		t.Fatalf("second run %v", res)
	}
	e.notifier.Wait()
	for user, want := range map[int64]string{230: "заканчивается", 231: "заканчивается", 232: "скрыт из поиска"} {
		if e.sender.countContaining(user, want) != 1 {
			t.Errorf("user %d: %v", user, e.sender.messagesTo(user))
		}
	}
	for _, user := range []int64{233, 234, 235} {
		if n := e.sender.countContaining(user, "Подписка"); n != 0 {
			t.Errorf("user %d should not be told: %v", user, e.sender.messagesTo(user))
		}
	}

	// The 1-day reminder still comes after the 7-day one.
	e.exec(`UPDATE subscriptions SET ends_at = now() + interval '10 hours' WHERE specialist_id = (SELECT id FROM specialists WHERE user_id = 230)`)
	if res := w.RunOnce(context.Background()); res["subscriptions_remind_1d"] != 1 {
		t.Fatalf("1d after 7d: %v", res)
	}

	var exp []struct {
		SpecialistID int64 `json:"specialist_id"`
	}
	e.mustCall(http.MethodGet, "/api/admin/subscriptions/expiring?days=7", adminID, nil, http.StatusOK, &exp)
	if len(exp) != 2 {
		t.Fatalf("expiring %+v, want users 230 and 231", exp)
	}
}

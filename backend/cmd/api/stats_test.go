package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

type weekJSON struct {
	WeekStart         string `json:"week_start"`
	Views             int    `json:"views"`
	SearchImpressions int    `json:"search_impressions"`
	Contacts          int    `json:"contacts"`
	Requests          int    `json:"requests"`
	Lessons           int    `json:"lessons"`
}

func TestStatsTracking(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(300)
	other := e.teacherWithSchedule(301)
	profile := fmt.Sprintf("/api/specialists/%d", id)

	e.mustCall(http.MethodGet, profile, 310, nil, http.StatusOK, nil)
	e.mustCall(http.MethodGet, profile, 311, nil, http.StatusOK, nil)
	e.mustCall(http.MethodGet, profile, 300, nil, http.StatusOK, nil)     // owner: not a view
	e.mustCall(http.MethodGet, profile, adminID, nil, http.StatusOK, nil) // admin: not a view
	e.search(310, "")                                                     // both teachers shown
	e.search(311, "q=nobody-matches")
	e.mustCall(http.MethodPost, profile+"/contact", 310, nil, http.StatusOK, nil)
	e.mustCall(http.MethodPost, profile+"/contact", 300, nil, http.StatusOK, nil) // own: not counted
	l := e.mustRequest(310, id, e.tomorrow(10, 0))
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/teacher/lessons/%d/confirm", l.ID), 300, nil, http.StatusOK, nil)
	e.mustRequest(311, id, e.tomorrow(12, 0))

	var st struct {
		Weeks  []weekJSON `json:"weeks"`
		Totals weekJSON   `json:"totals"`
	}
	e.mustCall(http.MethodGet, "/api/teacher/stats?weeks=4", 300, nil, http.StatusOK, &st)
	if len(st.Weeks) != 4 {
		t.Fatalf("weeks %d", len(st.Weeks))
	}
	now := time.Now().In(e.loc)
	monday := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7)).Format(time.DateOnly)
	cur := st.Weeks[3]
	if cur.WeekStart != monday || cur.Views != 2 || cur.SearchImpressions != 1 || cur.Contacts != 1 || cur.Requests != 2 {
		t.Fatalf("current week %+v (monday %s)", cur, monday)
	}
	// The confirmed lesson counts in the week it starts (tomorrow may be next week).
	if st.Totals.Lessons+boolInt(cur.WeekStart != e.tomorrow(0, 0).AddDate(0, 0, -((int(e.tomorrow(0, 0).Weekday())+6)%7)).Format(time.DateOnly)) != 1 {
		t.Fatalf("lessons %+v", st.Totals)
	}
	if st.Totals.Views != 2 || st.Weeks[0].WeekStart >= cur.WeekStart {
		t.Fatalf("totals %+v", st)
	}

	var otherStats struct {
		Totals weekJSON `json:"totals"`
	}
	e.mustCall(http.MethodGet, "/api/teacher/stats", 301, nil, http.StatusOK, &otherStats)
	if otherStats.Totals.SearchImpressions != 1 || otherStats.Totals.Views != 0 {
		t.Fatalf("other %+v (id %d)", otherStats.Totals, other)
	}
	e.wantError(e.call(http.MethodGet, "/api/teacher/stats", 310, nil), http.StatusNotFound, "not_found")
	e.wantError(e.call(http.MethodGet, "/api/teacher/stats?weeks=60", 300, nil), http.StatusBadRequest, "bad_request")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestContactLink(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(320, e.profileInput("C", "math", 5000))
	var c struct {
		TelegramURL string `json:"telegram_url"`
	}
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/specialists/%d/contact", id), 321, nil, http.StatusOK, &c)
	if c.TelegramURL != "https://t.me/dev_320" {
		t.Fatalf("url %q", c.TelegramURL)
	}
	e.exec(`UPDATE users SET username = NULL WHERE telegram_id = 320`)
	// The dev login refreshes the username, so read it as someone else.
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/specialists/%d/contact", id), 321, nil, http.StatusOK, &c)
	if c.TelegramURL != "tg://user?id=320" {
		t.Fatalf("url %q", c.TelegramURL)
	}
	e.mustCall(http.MethodPost, "/api/my/specialist/hide", 320, nil, http.StatusOK, nil)
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/specialists/%d/contact", id), 321, nil), http.StatusNotFound, "not_found")
}

func TestFavourites(t *testing.T) {
	e := newEnv(t)
	a := e.liveSpecialist(330, e.profileInput("A", "math", 5000))
	b := e.liveSpecialist(331, e.profileInput("B", "math", 5000))
	const u = 339
	e.mustCall(http.MethodPut, fmt.Sprintf("/api/favourites/%d", a), u, nil, http.StatusNoContent, nil)
	e.mustCall(http.MethodPut, fmt.Sprintf("/api/favourites/%d", a), u, nil, http.StatusNoContent, nil)
	e.mustCall(http.MethodPut, fmt.Sprintf("/api/favourites/%d", b), u, nil, http.StatusNoContent, nil)
	e.wantError(e.call(http.MethodPut, "/api/favourites/99999", u, nil), http.StatusNotFound, "not_found")

	var list []specialistJSON
	e.mustCall(http.MethodGet, "/api/favourites", u, nil, http.StatusOK, &list)
	if len(list) != 2 || list[0].ID != b {
		t.Fatalf("favourites %+v", list)
	}
	var p struct {
		Favourite bool `json:"favourite"`
	}
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d", a), u, nil, http.StatusOK, &p)
	if !p.Favourite {
		t.Fatal("profile does not show favourite")
	}
	e.mustCall(http.MethodDelete, fmt.Sprintf("/api/favourites/%d", a), u, nil, http.StatusNoContent, nil)
	e.mustCall(http.MethodDelete, fmt.Sprintf("/api/favourites/%d", a), u, nil, http.StatusNoContent, nil)
	// Hidden specialists drop out of the list.
	e.mustCall(http.MethodPost, "/api/my/specialist/hide", 331, nil, http.StatusOK, nil)
	e.mustCall(http.MethodGet, "/api/favourites", u, nil, http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("favourites %+v", list)
	}
}

func TestDashboard(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(340)
	e.mustCall(http.MethodPut, "/api/my/specialist", 341, e.profileInput("Draft", "math", 5000), http.StatusOK, nil)
	e.search(350, fmt.Sprintf("subject_id=%d", e.subjectID("chess")))
	e.search(351, fmt.Sprintf("subject_id=%d", e.subjectID("chess")))
	e.search(350, fmt.Sprintf("subject_id=%d", e.subjectID("math")))
	e.mustRequest(350, id, e.tomorrow(10, 0))

	var d struct {
		Users               int            `json:"users"`
		SpecialistsByStatus map[string]int `json:"specialists_by_status"`
		Listed              int            `json:"listed"`
		PlansRunning        map[string]int `json:"plans_running"`
		SearchesPerDay      []struct {
			Date  string `json:"date"`
			Count int    `json:"count"`
		} `json:"searches_per_day"`
		RequestsPerDay []struct {
			Count int `json:"count"`
		} `json:"requests_per_day"`
		UnmetDemand []struct {
			Slug     string `json:"slug"`
			Searches int    `json:"searches"`
			Listed   int    `json:"listed_specialists"`
		} `json:"unmet_demand"`
	}
	e.mustCall(http.MethodGet, "/api/admin/dashboard?days=7", adminID, nil, http.StatusOK, &d)
	today := time.Now().In(e.loc).Format(time.DateOnly)
	if d.SpecialistsByStatus["live"] != 1 || d.SpecialistsByStatus["draft"] != 1 || d.Listed != 1 || d.PlansRunning["trial"] != 1 {
		t.Fatalf("counts %+v", d)
	}
	if len(d.SearchesPerDay) != 7 || d.SearchesPerDay[6].Date != today || d.SearchesPerDay[6].Count != 3 || d.RequestsPerDay[6].Count != 1 {
		t.Fatalf("per day %+v %+v", d.SearchesPerDay, d.RequestsPerDay)
	}
	if len(d.UnmetDemand) != 2 || d.UnmetDemand[0].Slug != "chess" || d.UnmetDemand[0].Searches != 2 || d.UnmetDemand[0].Listed != 0 || d.UnmetDemand[1].Listed != 1 {
		t.Fatalf("unmet demand %+v", d.UnmetDemand)
	}
	if d.Users < 5 {
		t.Fatalf("users %d", d.Users)
	}
	e.wantError(e.call(http.MethodGet, "/api/admin/dashboard", 350, nil), http.StatusForbidden, "forbidden")
}

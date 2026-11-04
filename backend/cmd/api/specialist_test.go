package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

const adminID = 900

type specialistJSON struct {
	ID              int64   `json:"id"`
	Status          string  `json:"status"`
	DisplayName     string  `json:"display_name"`
	PhotoURL        *string `json:"photo_url"`
	Listed          bool    `json:"listed"`
	Featured        bool    `json:"featured"`
	Address         *string `json:"address"`
	RejectionReason *string `json:"rejection_reason"`
	PersonalLink    *string `json:"personal_link"`
	MinPriceKZT     *int    `json:"min_price_kzt"`
	Subscription    *struct {
		Plan   string    `json:"plan"`
		EndsAt time.Time `json:"ends_at"`
	} `json:"subscription"`
	Subjects []struct {
		SubjectID int64    `json:"subject_id"`
		Slug      string   `json:"slug"`
		Levels    []string `json:"levels"`
		PriceKZT  int      `json:"price_kzt"`
	} `json:"subjects"`
}

type pageJSON struct {
	Items []specialistJSON `json:"items"`
	Total int              `json:"total"`
}

func (e *env) subjectID(slug string) int64 {
	e.t.Helper()
	return e.id(`SELECT id FROM subjects WHERE slug = $1`, slug)
}

// profileInput is a complete, submittable profile teaching `slug` at `price`.
func (e *env) profileInput(name, slug string, price int, mutate ...func(map[string]any)) map[string]any {
	in := map[string]any{
		"kind":             "teacher",
		"display_name":     name,
		"bio":              "Опытный преподаватель, готовлю к ЕНТ и олимпиадам, индивидуальный подход.",
		"education":        "КазНУ, мехмат",
		"experience_years": 5,
		"district":         "bostandyk",
		"address":          "ул. Тимирязева 42, кв. 7",
		"formats":          []string{"online", "teacher_place"},
		"languages":        []string{"ru", "kk"},
		"trial_kind":       "free",
		"lesson_minutes":   60,
		"subjects": []map[string]any{
			{"subject_id": e.subjectID(slug), "levels": []string{"high", "ent"}, "price_kzt": price},
		},
	}
	for _, m := range mutate {
		m(in)
	}
	return in
}

// liveSpecialist creates, submits and approves a profile for user; returns its id.
func (e *env) liveSpecialist(user int64, in map[string]any) int64 {
	e.t.Helper()
	e.user(adminID, true)
	var p specialistJSON
	e.mustCall(http.MethodPut, "/api/my/specialist", user, in, http.StatusOK, &p)
	e.mustCall(http.MethodPost, "/api/my/specialist/submit", user, nil, http.StatusOK, nil)
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/approve", p.ID), adminID, nil, http.StatusOK, nil)
	return p.ID
}

func (e *env) search(user int64, query string) pageJSON {
	e.t.Helper()
	var page pageJSON
	e.mustCall(http.MethodGet, "/api/specialists?"+query, user, nil, http.StatusOK, &page)
	return page
}

func ids(p pageJSON) []int64 {
	out := []int64{}
	for _, it := range p.Items {
		out = append(out, it.ID)
	}
	return out
}

func TestProfileLifecycle(t *testing.T) {
	e := newEnv(t)
	e.user(adminID, true)
	const teacher = 101

	e.wantError(e.call(http.MethodGet, "/api/my/specialist", teacher, nil), http.StatusNotFound, "not_found")

	// A bare draft can be saved, but not submitted.
	var p specialistJSON
	e.mustCall(http.MethodPut, "/api/my/specialist", teacher, map[string]any{"display_name": "Айгерим"}, http.StatusOK, &p)
	if p.Status != "draft" || p.Listed || p.Subscription != nil {
		t.Fatalf("draft = %+v", p)
	}
	r := e.call(http.MethodPost, "/api/my/specialist/submit", teacher, nil)
	e.wantError(r, http.StatusUnprocessableEntity, "profile_incomplete")
	if !strings.Contains(string(r.body), "subjects") {
		t.Errorf("message should list subjects: %s", r.body)
	}

	e.mustCall(http.MethodPut, "/api/my/specialist", teacher, e.profileInput("Айгерим", "ent-math", 6000), http.StatusOK, &p)
	if len(p.Subjects) != 1 || p.Subjects[0].Slug != "ent-math" || *p.MinPriceKZT != 6000 {
		t.Fatalf("saved = %+v", p)
	}
	e.mustCall(http.MethodPost, "/api/my/specialist/submit", teacher, nil, http.StatusOK, &p)
	if p.Status != "pending" {
		t.Fatalf("status %s", p.Status)
	}
	e.notifier.Wait()
	if e.sender.countContaining(adminID, "Новая анкета") != 1 {
		t.Errorf("admin not notified: %v", e.sender.messagesTo(adminID))
	}
	e.wantError(e.call(http.MethodPost, "/api/my/specialist/submit", teacher, nil), http.StatusConflict, "invalid_status")

	// Only admins approve.
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/approve", p.ID), teacher, nil), http.StatusForbidden, "forbidden")
	var queue []specialistJSON
	e.mustCall(http.MethodGet, "/api/admin/specialists", adminID, nil, http.StatusOK, &queue)
	if len(queue) != 1 || queue[0].ID != p.ID {
		t.Fatalf("queue %+v", queue)
	}

	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/approve", p.ID), adminID, nil, http.StatusOK, &p)
	if p.Status != "live" || !p.Listed || p.Subscription == nil || p.Subscription.Plan != "trial" {
		t.Fatalf("approved = %+v", p)
	}
	if days := time.Until(p.Subscription.EndsAt).Hours() / 24; days < 29 || days > 32 {
		t.Errorf("trial ends in %.1f days, want ~30", days)
	}
	if p.PersonalLink == nil || *p.PersonalLink != fmt.Sprintf("https://t.me/tutorhub_test_bot?start=t_%d", p.ID) {
		t.Errorf("personal link %v", p.PersonalLink)
	}
	e.notifier.Wait()
	if e.sender.countContaining(teacher, "одобрен") != 1 {
		t.Errorf("teacher not notified: %v", e.sender.messagesTo(teacher))
	}
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/approve", p.ID), adminID, nil), http.StatusConflict, "invalid_status")

	var me struct {
		Specialist *struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
			Listed bool   `json:"listed"`
		} `json:"specialist"`
	}
	e.mustCall(http.MethodGet, "/api/me", teacher, nil, http.StatusOK, &me)
	if me.Specialist == nil || me.Specialist.ID != p.ID || !me.Specialist.Listed {
		t.Fatalf("me.specialist = %+v", me.Specialist)
	}

	// Editing a live profile keeps it live; hide/unhide.
	e.mustCall(http.MethodPut, "/api/my/specialist", teacher, e.profileInput("Айгерим Б.", "ent-math", 7000), http.StatusOK, &p)
	if p.Status != "live" {
		t.Fatalf("edit changed status to %s", p.Status)
	}
	e.mustCall(http.MethodPost, "/api/my/specialist/hide", teacher, nil, http.StatusOK, &p)
	if p.Status != "hidden" || p.Listed {
		t.Fatalf("hidden = %+v", p)
	}
	e.wantError(e.call(http.MethodPost, "/api/my/specialist/hide", teacher, nil), http.StatusConflict, "invalid_status")
	e.mustCall(http.MethodPost, "/api/my/specialist/unhide", teacher, nil, http.StatusOK, &p)
	if p.Status != "live" || !p.Listed {
		t.Fatalf("unhidden = %+v", p)
	}
}

func TestRejectAndFoundingApproval(t *testing.T) {
	e := newEnv(t)
	e.user(adminID, true)
	var p specialistJSON
	e.mustCall(http.MethodPut, "/api/my/specialist", 102, e.profileInput("Ерлан", "physics", 5000), http.StatusOK, &p)
	e.mustCall(http.MethodPost, "/api/my/specialist/submit", 102, nil, http.StatusOK, nil)

	path := fmt.Sprintf("/api/admin/specialists/%d", p.ID)
	e.wantError(e.call(http.MethodPost, path+"/reject", adminID, map[string]any{"reason": "  "}), http.StatusBadRequest, "bad_request")
	e.mustCall(http.MethodPost, path+"/reject", adminID, map[string]any{"reason": "Добавьте фото и образование"}, http.StatusOK, &p)
	if p.Status != "draft" || p.RejectionReason == nil || *p.RejectionReason != "Добавьте фото и образование" {
		t.Fatalf("rejected = %+v", p)
	}
	e.notifier.Wait()
	if e.sender.countContaining(102, "Добавьте фото") != 1 {
		t.Errorf("teacher not told the reason: %v", e.sender.messagesTo(102))
	}

	e.mustCall(http.MethodPost, "/api/my/specialist/submit", 102, nil, http.StatusOK, &p)
	if p.RejectionReason != nil {
		t.Errorf("resubmission keeps rejection_reason %v", *p.RejectionReason)
	}
	e.wantError(e.call(http.MethodPost, path+"/approve", adminID, map[string]any{"plan": "founding"}), http.StatusBadRequest, "bad_request")
	e.wantError(e.call(http.MethodPost, path+"/approve", adminID, map[string]any{"plan": "featured", "ends_on": "2030-01-01"}), http.StatusBadRequest, "bad_request")
	e.wantError(e.call(http.MethodPost, path+"/approve", adminID, map[string]any{"plan": "founding", "ends_on": "2020-01-01"}), http.StatusBadRequest, "bad_request")
	e.mustCall(http.MethodPost, path+"/approve", adminID, map[string]any{"plan": "founding", "ends_on": "2030-06-30"}, http.StatusOK, &p)
	want := time.Date(2030, 7, 1, 0, 0, 0, 0, e.loc)
	if p.Subscription == nil || p.Subscription.Plan != "founding" || !p.Subscription.EndsAt.Equal(want) {
		t.Fatalf("subscription %+v, want founding until %v", p.Subscription, want)
	}
	e.wantError(e.call(http.MethodPost, "/api/admin/specialists/99999/approve", adminID, nil), http.StatusNotFound, "not_found")
}

// The visibility rule: live AND a running subscription. Expired teachers leave search
// but their link keeps working; drafts and pending profiles are private.
func TestVisibilityRule(t *testing.T) {
	e := newEnv(t)
	const viewer = 300
	live := e.liveSpecialist(201, e.profileInput("Live", "english", 5000))
	expired := e.liveSpecialist(202, e.profileInput("Expired", "english", 5000))
	e.exec(`UPDATE subscriptions SET starts_at = now() - interval '40 days', ends_at = now() - interval '1 day' WHERE specialist_id = $1`, expired)
	future := e.liveSpecialist(203, e.profileInput("Future", "english", 5000))
	e.exec(`UPDATE subscriptions SET starts_at = now() + interval '1 day', ends_at = now() + interval '30 days' WHERE specialist_id = $1`, future)

	var draft, pending specialistJSON
	e.mustCall(http.MethodPut, "/api/my/specialist", 204, e.profileInput("Draft", "english", 5000), http.StatusOK, &draft)
	e.mustCall(http.MethodPut, "/api/my/specialist", 205, e.profileInput("Pending", "english", 5000), http.StatusOK, &pending)
	e.mustCall(http.MethodPost, "/api/my/specialist/submit", 205, nil, http.StatusOK, nil)
	hidden := e.liveSpecialist(206, e.profileInput("Hidden", "english", 5000))
	e.mustCall(http.MethodPost, "/api/my/specialist/hide", 206, nil, http.StatusOK, nil)

	page := e.search(viewer, "")
	if got := ids(page); len(got) != 1 || got[0] != live || page.Total != 1 {
		t.Fatalf("search = %v (total %d), want only %d", got, page.Total, live)
	}

	var p specialistJSON
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d", expired), viewer, nil, http.StatusOK, &p)
	if p.Listed {
		t.Error("expired specialist reads as listed")
	}
	for _, id := range []int64{draft.ID, pending.ID, hidden} {
		e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/specialists/%d", id), viewer, nil), http.StatusNotFound, "not_found")
	}
	// Owner and admin still see private profiles.
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d", draft.ID), 204, nil, http.StatusOK, nil)
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d", pending.ID), adminID, nil, http.StatusOK, nil)

	// Renewing brings the expired teacher back instantly.
	e.exec(`INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at) VALUES ($1, 'basic', now(), now() + interval '30 days')`, expired)
	if got := ids(e.search(viewer, "")); len(got) != 2 {
		t.Fatalf("after renewal search = %v", got)
	}
}

func TestAddressOnlyForOwnerAndAdmin(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(210, e.profileInput("Addr", "chemistry", 4000))
	path := fmt.Sprintf("/api/specialists/%d", id)
	var p specialistJSON
	e.mustCall(http.MethodGet, path, 999, nil, http.StatusOK, &p)
	if p.Address != nil {
		t.Errorf("stranger sees address %q", *p.Address)
	}
	for _, u := range []int64{210, adminID} {
		e.mustCall(http.MethodGet, path, u, nil, http.StatusOK, &p)
		if p.Address == nil || *p.Address == "" {
			t.Errorf("user %d does not see the address", u)
		}
	}
}

func TestSearchFilters(t *testing.T) {
	e := newEnv(t)
	math := e.liveSpecialist(401, e.profileInput("Айжан Математика", "ent-math", 6000))
	eng := e.liveSpecialist(402, e.profileInput("John English", "ielts", 12000, func(in map[string]any) {
		in["district"] = "medeu"
		in["formats"] = []string{"online"}
		in["languages"] = []string{"en", "ru"}
		in["subjects"] = []map[string]any{{"subject_id": e.subjectID("ielts"), "levels": []string{"adult", "university"}, "price_kzt": 12000}}
	}))
	cons := e.liveSpecialist(403, e.profileInput("Career Pro", "career", 20000, func(in map[string]any) {
		in["kind"] = "consultant"
		in["district"] = "almaly"
		in["formats"] = []string{"student_home"}
		in["languages"] = []string{"kk"}
		in["subjects"] = []map[string]any{
			{"subject_id": e.subjectID("career"), "levels": []string{"adult"}, "price_kzt": 20000},
			{"subject_id": e.subjectID("ent-math"), "levels": []string{"primary"}, "price_kzt": 3000},
		}
	}))
	e.exec(`UPDATE specialists SET verified = true WHERE id = $1`, eng)
	const u = 500

	tests := []struct {
		query string
		want  []int64
	}{
		{"subject_id=" + fmt.Sprint(e.subjectID("ent-math")), []int64{cons, math}},
		{"subject_id=" + fmt.Sprint(e.subjectID("ent-math")) + "&level=ent", []int64{math}},
		// Price must match the same subject: the consultant's cheap math is primary-level only.
		{"subject_id=" + fmt.Sprint(e.subjectID("ent-math")) + "&price_max=5000", []int64{cons}},
		{"subject_id=" + fmt.Sprint(e.subjectID("ent-math")) + "&level=ent&price_max=5000", []int64{}},
		{"price_min=10000", []int64{cons, eng}},
		{"language=en", []int64{eng}},
		{"format=student_home", []int64{cons}},
		{"format=online", []int64{eng, math}},
		{"kind=consultant", []int64{cons}},
		{"city=almaty&district=medeu", []int64{eng}},
		{"verified=true", []int64{eng}},
		{"q=english", []int64{eng}},
		{"q=" + "%D0%BC%D0%B0%D1%82%D0%B5%D0%BC%D0%B0%D1%82", []int64{cons, math}}, // "математ" matches a name and a subject
		{"q=100%25", []int64{}},
		{"limit=1", []int64{cons}},
		{"limit=1&offset=1", []int64{eng}},
	}
	for _, tt := range tests {
		got := ids(e.search(u, tt.query))
		if fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.query, got, tt.want)
		}
	}
	if p := e.search(u, "limit=1&offset=1"); p.Total != 3 {
		t.Errorf("total = %d, want 3", p.Total)
	}
	if p := e.search(u, "offset=50"); p.Total != 3 || len(p.Items) != 0 {
		t.Errorf("past the end: %+v", p)
	}

	// Featured ranks first.
	e.exec(`INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at) VALUES ($1, 'featured', now(), now() + interval '30 days')`, math)
	page := e.search(u, "")
	if got := ids(page); got[0] != math || !page.Items[0].Featured {
		t.Errorf("featured not first: %v", got)
	}

	for _, bad := range []string{"level=phd", "price_max=-1", "limit=0", "limit=51", "district=soho", "verified=maybe", "subject_id=abc"} {
		e.wantError(e.call(http.MethodGet, "/api/specialists?"+bad, u, nil), http.StatusBadRequest, "bad_request")
	}
}

func TestProfileValidation(t *testing.T) {
	e := newEnv(t)
	const u = 600
	bad := []func(map[string]any){
		func(in map[string]any) { in["kind"] = "wizard" },
		func(in map[string]any) { in["formats"] = []string{"online", "online"} },
		func(in map[string]any) { in["languages"] = []string{"de"} },
		func(in map[string]any) { in["district"] = "soho" },
		func(in map[string]any) { in["trial_kind"] = "paid" },
		func(in map[string]any) { in["lesson_minutes"] = 50 },
		func(in map[string]any) { in["display_name"] = strings.Repeat("я", 81) },
		func(in map[string]any) {
			in["subjects"] = []map[string]any{{"subject_id": 999999, "levels": []string{"adult"}, "price_kzt": 1}}
		},
		func(in map[string]any) {
			s := map[string]any{"subject_id": e.subjectID("math"), "levels": []string{"adult"}, "price_kzt": 1}
			in["subjects"] = []map[string]any{s, s}
		},
		func(in map[string]any) {
			in["subjects"] = []map[string]any{{"subject_id": e.subjectID("math"), "levels": []string{"phd"}, "price_kzt": 1}}
		},
	}
	for i, m := range bad {
		r := e.call(http.MethodPut, "/api/my/specialist", u, e.profileInput("V", "math", 1000, m))
		if r.code != http.StatusBadRequest {
			t.Errorf("case %d: %d %s", i, r.code, r.body)
		}
	}
	// Paid trial needs a price, which is dropped for other trial kinds.
	var p struct {
		TrialPrice *int `json:"trial_price"`
	}
	e.mustCall(http.MethodPut, "/api/my/specialist", u, e.profileInput("V", "math", 1000, func(in map[string]any) {
		in["trial_kind"] = "paid"
		in["trial_price"] = 2000
	}), http.StatusOK, &p)
	if p.TrialPrice == nil || *p.TrialPrice != 2000 {
		t.Fatalf("trial_price = %v", p.TrialPrice)
	}
	e.mustCall(http.MethodPut, "/api/my/specialist", u, e.profileInput("V", "math", 1000, func(in map[string]any) {
		in["trial_price"] = 2000
	}), http.StatusOK, &p)
	if p.TrialPrice != nil {
		t.Fatalf("free trial kept price %d", *p.TrialPrice)
	}

	// A blocked profile cannot be edited.
	e.exec(`UPDATE specialists SET status = 'blocked' WHERE user_id = $1`, u)
	e.wantError(e.call(http.MethodPut, "/api/my/specialist", u, e.profileInput("V", "math", 1000)), http.StatusForbidden, "forbidden")
}

func pngBytes(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPhotoUpload(t *testing.T) {
	e := newEnv(t)
	const u = 700
	e.wantError(e.upload("/api/my/specialist/photo", u, "a.png", pngBytes(t)), http.StatusNotFound, "not_found")
	e.mustCall(http.MethodPut, "/api/my/specialist", u, map[string]any{"display_name": "Photo"}, http.StatusOK, nil)

	e.wantError(e.upload("/api/my/specialist/photo", u, "a.txt", []byte("hello, not an image")), http.StatusUnprocessableEntity, "invalid_photo")

	var p specialistJSON
	r := e.upload("/api/my/specialist/photo", u, "a.png", pngBytes(t))
	if r.code != http.StatusOK {
		t.Fatalf("upload: %d %s", r.code, r.body)
	}
	if err := json.Unmarshal(r.body, &p); err != nil || p.PhotoURL == nil || !strings.HasPrefix(*p.PhotoURL, "/api/files/photos/") {
		t.Fatalf("photo_url = %v (%v)", p.PhotoURL, err)
	}
	first := *p.PhotoURL

	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, first, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("GET photo: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}

	// Replacing deletes the old file.
	r = e.upload("/api/my/specialist/photo", u, "b.png", pngBytes(t))
	_ = json.Unmarshal(r.body, &p)
	if *p.PhotoURL == first {
		t.Fatal("photo url did not change")
	}
	if _, err := os.Stat(filepath.Join(e.app.cfg.UploadDir, "photos", strings.TrimPrefix(first, "/api/files/photos/"))); !os.IsNotExist(err) {
		t.Errorf("old photo still on disk: %v", err)
	}
	for _, bad := range []string{"/api/files/photos/../../etc/passwd", "/api/files/photos/1-0000000000000000.png"} {
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, bad, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
}

func TestMeta(t *testing.T) {
	e := newEnv(t)
	var m struct {
		Cities []struct {
			Slug      string `json:"slug"`
			Districts []any  `json:"districts"`
		} `json:"cities"`
	}
	e.mustCall(http.MethodGet, "/api/meta", 1, nil, http.StatusOK, &m)
	if len(m.Cities) != 1 || m.Cities[0].Slug != "almaty" || len(m.Cities[0].Districts) != 8 {
		t.Fatalf("meta %+v", m)
	}
}

func TestBotTeachAndPersonalLink(t *testing.T) {
	e := newEnv(t)
	msg := func(from int64, text string) *models.Update {
		return &models.Update{Message: &models.Message{Text: text, Chat: models.Chat{ID: from, Type: models.ChatTypePrivate}, From: &models.User{ID: from}}}
	}
	ctx := context.Background()

	e.bot.HandleUpdate(ctx, msg(800, "/teach"))
	if e.sender.countContaining(800, "Станьте преподавателем") != 1 {
		t.Fatalf("new teacher: %v", e.sender.messagesTo(800))
	}

	id := e.liveSpecialist(801, e.profileInput("Динара", "piano", 7000))
	e.bot.HandleUpdate(ctx, msg(801, "/teach"))
	if e.sender.countContaining(801, fmt.Sprintf("start=t_%d", id)) != 1 {
		t.Fatalf("live teacher should get the personal link: %v", e.sender.messagesTo(801))
	}

	e.bot.HandleUpdate(ctx, msg(802, fmt.Sprintf("/start t_%d", id)))
	if e.sender.countContaining(802, "Динара") != 1 {
		t.Fatalf("personal link: %v", e.sender.messagesTo(802))
	}
	e.bot.HandleUpdate(ctx, msg(803, "/start t_999999"))
	if e.sender.countContaining(803, "Добро пожаловать") != 1 {
		t.Fatalf("unknown link falls back to welcome: %v", e.sender.messagesTo(803))
	}
}

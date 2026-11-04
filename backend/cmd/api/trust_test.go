package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReviews(t *testing.T) {
	e := newEnv(t)
	id := e.teacherWithSchedule(400)
	path := fmt.Sprintf("/api/specialists/%d/review", id)

	var mine struct {
		CanReview bool `json:"can_review"`
		Review    *struct {
			Rating     int    `json:"rating"`
			AuthorName string `json:"author_name"`
		} `json:"review"`
	}
	e.mustCall(http.MethodGet, path, 401, nil, http.StatusOK, &mine)
	if mine.CanReview || mine.Review != nil {
		t.Fatalf("before a lesson %+v", mine)
	}
	e.wantError(e.call(http.MethodPut, path, 401, map[string]any{"rating": 5}), http.StatusUnprocessableEntity, "no_completed_lesson")
	// A confirmed lesson that has not happened yet is not enough.
	e.insertLesson(id, 401, e.tomorrow(10, 0), "confirmed", nil)
	e.wantError(e.call(http.MethodPut, path, 401, map[string]any{"rating": 5}), http.StatusUnprocessableEntity, "no_completed_lesson")
	// A confirmed lesson that ended counts (effective status completed).
	e.insertLesson(id, 401, time.Now().Add(-3*time.Hour), "confirmed", nil)
	e.insertLesson(id, 402, time.Now().Add(-26*time.Hour), "completed", nil)

	e.mustCall(http.MethodGet, path, 401, nil, http.StatusOK, &mine)
	if !mine.CanReview {
		t.Fatal("can_review should be true after a completed lesson")
	}
	e.wantError(e.call(http.MethodPut, path, 401, map[string]any{"rating": 6}), http.StatusBadRequest, "bad_request")
	e.mustCall(http.MethodPut, path, 401, map[string]any{"rating": 5, "text": "Отличный учитель!"}, http.StatusOK, nil)
	e.mustCall(http.MethodPut, path, 401, map[string]any{"rating": 4, "text": "Хороший учитель"}, http.StatusOK, nil) // edit
	e.mustCall(http.MethodPut, path, 402, map[string]any{"rating": 3}, http.StatusOK, nil)
	e.notifier.Wait()
	if n := e.sender.countContaining(400, "Новый отзыв"); n != 2 {
		t.Errorf("teacher told %d times, want 2 (edits are silent): %v", n, e.sender.messagesTo(400))
	}

	var list []struct {
		Rating     int    `json:"rating"`
		Text       string `json:"text"`
		AuthorName string `json:"author_name"`
	}
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d/reviews", id), 999, nil, http.StatusOK, &list)
	if len(list) != 2 || list[1].Rating != 4 || list[1].AuthorName != "Dev" {
		t.Fatalf("reviews %+v", list)
	}
	var p struct {
		Rating       *float64 `json:"rating"`
		ReviewsCount int      `json:"reviews_count"`
	}
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d", id), 999, nil, http.StatusOK, &p)
	if p.Rating == nil || *p.Rating != 3.5 || p.ReviewsCount != 2 {
		t.Fatalf("rating %v count %d", p.Rating, p.ReviewsCount)
	}

	// Moderation: hiding drops the review from the list and the rating.
	var adminList []struct {
		ID     int64  `json:"id"`
		Rating int    `json:"rating"`
		Status string `json:"status"`
	}
	e.mustCall(http.MethodGet, "/api/admin/reviews", adminID, nil, http.StatusOK, &adminList)
	if len(adminList) != 2 {
		t.Fatalf("admin reviews %+v", adminList)
	}
	var low int64
	for _, r := range adminList {
		if r.Rating == 3 {
			low = r.ID
		}
	}
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/reviews/%d/hide", low), adminID, nil, http.StatusOK, nil)
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d", id), 999, nil, http.StatusOK, &p)
	if *p.Rating != 4 || p.ReviewsCount != 1 {
		t.Fatalf("after hide: rating %v count %d", *p.Rating, p.ReviewsCount)
	}
	e.wantError(e.call(http.MethodPost, "/api/admin/reviews/99999/hide", adminID, nil), http.StatusNotFound, "not_found")
	e.wantError(e.call(http.MethodGet, "/api/admin/reviews", 401, nil), http.StatusForbidden, "forbidden")

	e.mustCall(http.MethodDelete, path, 401, nil, http.StatusNoContent, nil)
	e.mustCall(http.MethodGet, fmt.Sprintf("/api/specialists/%d/reviews", id), 999, nil, http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("after delete %+v", list)
	}
}

func TestRatingFilterAndRanking(t *testing.T) {
	e := newEnv(t)
	good := e.liveSpecialist(410, e.profileInput("Good", "math", 5000))
	bad := e.liveSpecialist(411, e.profileInput("Bad", "math", 5000))
	none := e.liveSpecialist(412, e.profileInput("None", "math", 5000))
	review := func(sid, student int64, rating int) {
		l := e.insertLesson(sid, student, time.Now().Add(-48*time.Hour), "completed", nil)
		e.exec(`INSERT INTO reviews (specialist_id, student_id, lesson_id, rating) VALUES ($1, $2, $3, $4)`, sid, student, l, rating)
	}
	review(good, 420, 5)
	review(bad, 421, 2)

	if got := ids(e.search(1, "")); fmt.Sprint(got) != fmt.Sprint([]int64{good, bad, none}) {
		t.Fatalf("ranking %v, want good, bad, then unrated", got)
	}
	if got := ids(e.search(1, "rating_min=4")); fmt.Sprint(got) != fmt.Sprint([]int64{good}) {
		t.Fatalf("rating_min=4: %v", got)
	}
	e.wantError(e.call(http.MethodGet, "/api/specialists?rating_min=6", 1, nil), http.StatusBadRequest, "bad_request")
}

func TestReports(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(430, e.profileInput("R", "math", 5000))
	path := fmt.Sprintf("/api/specialists/%d/report", id)
	var r1, r2 struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	e.mustCall(http.MethodPost, path, 431, map[string]any{"reason": "fake", "text": "Фото из интернета"}, http.StatusCreated, &r1)
	e.mustCall(http.MethodPost, path, 431, map[string]any{"reason": "spam"}, http.StatusCreated, &r2)
	if r1.ID != r2.ID || r1.Status != "open" {
		t.Fatalf("repeat report should return the open one: %+v %+v", r1, r2)
	}
	e.wantError(e.call(http.MethodPost, path, 431, map[string]any{"reason": "boring"}), http.StatusBadRequest, "bad_request")
	e.notifier.Wait()
	if e.sender.countContaining(adminID, "Жалоба") != 1 {
		t.Errorf("admin not told once: %v", e.sender.messagesTo(adminID))
	}

	var open []struct {
		ID int64 `json:"id"`
	}
	e.mustCall(http.MethodGet, "/api/admin/reports", adminID, nil, http.StatusOK, &open)
	if len(open) != 1 {
		t.Fatalf("open reports %+v", open)
	}
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/reports/%d/resolve", r1.ID), adminID, map[string]any{"status": "dismissed"}, http.StatusOK, &r1)
	if r1.Status != "dismissed" {
		t.Fatalf("resolved %+v", r1)
	}
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/admin/reports/%d/resolve", r1.ID), adminID, map[string]any{"status": "open"}), http.StatusBadRequest, "bad_request")
	// A new report is possible once the old one is closed.
	e.mustCall(http.MethodPost, path, 431, map[string]any{"reason": "spam"}, http.StatusCreated, &r2)
	if r2.ID == r1.ID {
		t.Fatal("expected a new report")
	}
}

func TestBlocking(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(440, e.profileInput("B", "math", 5000))

	// Block a profile: gone from search and its page, the owner sees the reason.
	var p specialistJSON
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/block", id), adminID, map[string]any{"reason": ""}), http.StatusBadRequest, "bad_request")
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/block", id), adminID, map[string]any{"reason": "Жалобы учеников"}, http.StatusOK, &p)
	if p.Status != "blocked" {
		t.Fatalf("status %s", p.Status)
	}
	if got := ids(e.search(1, "")); len(got) != 0 {
		t.Fatalf("blocked still listed %v", got)
	}
	e.wantError(e.call(http.MethodGet, fmt.Sprintf("/api/specialists/%d", id), 441, nil), http.StatusNotFound, "not_found")
	var mine struct {
		BlockReason string `json:"block_reason"`
	}
	e.mustCall(http.MethodGet, "/api/my/specialist", 440, nil, http.StatusOK, &mine)
	if mine.BlockReason != "Жалобы учеников" {
		t.Fatalf("block reason %q", mine.BlockReason)
	}
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/block", id), adminID, map[string]any{"reason": "x"}), http.StatusConflict, "invalid_status")
	e.mustCall(http.MethodPost, fmt.Sprintf("/api/admin/specialists/%d/unblock", id), adminID, nil, http.StatusOK, &p)
	if p.Status != "live" || !p.Listed {
		t.Fatalf("after unblock %+v", p)
	}

	// Block a user: every request is 403 until unblocked; admins cannot be blocked.
	e.mustCall(http.MethodGet, "/api/me", 441, nil, http.StatusOK, nil)
	e.mustCall(http.MethodPost, "/api/admin/users/441/block", adminID, nil, http.StatusNoContent, nil)
	e.wantError(e.call(http.MethodGet, "/api/me", 441, nil), http.StatusForbidden, "forbidden")
	e.wantError(e.call(http.MethodGet, "/api/specialists", 441, nil), http.StatusForbidden, "forbidden")
	e.mustCall(http.MethodPost, "/api/admin/users/441/unblock", adminID, nil, http.StatusNoContent, nil)
	e.mustCall(http.MethodGet, "/api/me", 441, nil, http.StatusOK, nil)
	e.wantError(e.call(http.MethodPost, fmt.Sprintf("/api/admin/users/%d/block", adminID), adminID, nil), http.StatusNotFound, "not_found")
	e.wantError(e.call(http.MethodPost, "/api/admin/users/123456789/block", adminID, nil), http.StatusNotFound, "not_found")
}

func pdfBytes() []byte {
	return []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << >>\n%%EOF\n")
}

func TestVerificationFlow(t *testing.T) {
	e := newEnv(t)
	id := e.liveSpecialist(450, e.profileInput("V", "math", 5000))

	e.wantError(e.call(http.MethodPost, "/api/my/specialist/verification", 450, nil), http.StatusUnprocessableEntity, "no_documents")
	e.wantError(e.upload("/api/my/specialist/documents", 450, "notes.txt", []byte("just text")), http.StatusUnprocessableEntity, "invalid_document")
	r := e.upload("/api/my/specialist/documents", 450, "diploma.pdf", pdfBytes())
	if r.code != http.StatusCreated {
		t.Fatalf("upload: %d %s", r.code, r.body)
	}
	var doc struct {
		ID           int64  `json:"id"`
		OriginalName string `json:"original_name"`
		ContentType  string `json:"content_type"`
	}
	_ = json.Unmarshal(r.body, &doc)
	if doc.OriginalName != "diploma.pdf" || doc.ContentType != "application/pdf" {
		t.Fatalf("doc %+v", doc)
	}
	extra := e.upload("/api/my/specialist/documents", 450, "cert.png", pngBytes(t))
	var extraDoc struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(extra.body, &extraDoc)
	e.mustCall(http.MethodDelete, fmt.Sprintf("/api/my/specialist/documents/%d", extraDoc.ID), 450, nil, http.StatusNoContent, nil)
	e.wantError(e.call(http.MethodDelete, fmt.Sprintf("/api/my/specialist/documents/%d", doc.ID), 451, nil), http.StatusNotFound, "not_found")

	// Only admins can read documents.
	file := fmt.Sprintf("/api/admin/documents/%d/file", doc.ID)
	e.wantError(e.call(http.MethodGet, file, 450, nil), http.StatusForbidden, "forbidden")
	e.wantError(e.call(http.MethodGet, file, 451, nil), http.StatusForbidden, "forbidden")
	req := httptest.NewRequest(http.MethodGet, file, nil)
	req.Header.Set("Authorization", fmt.Sprintf("dev %d", adminID))
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" || !bytes.Equal(rec.Body.Bytes(), pdfBytes()) {
		t.Fatalf("admin download: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}

	var p struct {
		Verified           bool   `json:"verified"`
		VerificationStatus string `json:"verification_status"`
		VerificationNote   string `json:"verification_note"`
	}
	e.mustCall(http.MethodPost, "/api/my/specialist/verification", 450, nil, http.StatusOK, &p)
	if p.VerificationStatus != "requested" {
		t.Fatalf("after request %+v", p)
	}
	e.wantError(e.call(http.MethodPost, "/api/my/specialist/verification", 450, nil), http.StatusConflict, "invalid_status")

	var queue []struct {
		Specialist struct {
			ID int64 `json:"id"`
		} `json:"specialist"`
		Documents []struct {
			ID int64 `json:"id"`
		} `json:"documents"`
	}
	e.mustCall(http.MethodGet, "/api/admin/verifications", adminID, nil, http.StatusOK, &queue)
	if len(queue) != 1 || queue[0].Specialist.ID != id || len(queue[0].Documents) != 1 {
		t.Fatalf("queue %+v", queue)
	}
	vpath := fmt.Sprintf("/api/admin/specialists/%d/verification", id)
	e.wantError(e.call(http.MethodPost, vpath, adminID, map[string]any{"note": "x"}), http.StatusBadRequest, "bad_request")
	e.mustCall(http.MethodPost, vpath, adminID, map[string]any{"approve": false, "note": "Документ нечитаем"}, http.StatusOK, &p)
	if p.Verified || p.VerificationStatus != "rejected" || p.VerificationNote != "Документ нечитаем" {
		t.Fatalf("rejected %+v", p)
	}
	e.mustCall(http.MethodPost, "/api/my/specialist/verification", 450, nil, http.StatusOK, nil) // may ask again
	e.mustCall(http.MethodPost, vpath, adminID, map[string]any{"approve": true}, http.StatusOK, &p)
	if !p.Verified || p.VerificationStatus != "verified" {
		t.Fatalf("verified %+v", p)
	}
	e.notifier.Wait()
	if e.sender.countContaining(450, "Проверен") != 1 || e.sender.countContaining(450, "нечитаем") != 1 {
		t.Errorf("teacher messages %v", e.sender.messagesTo(450))
	}
	if got := ids(e.search(1, "verified=true")); fmt.Sprint(got) != fmt.Sprint([]int64{id}) {
		t.Fatalf("verified filter %v", got)
	}
}

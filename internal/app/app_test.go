package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/store"
	"github.com/papergrade/papergrade/internal/vision"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

type harness struct {
	t      *testing.T
	a      *App
	h      http.Handler
	cookie *http.Cookie
}

func setup(t *testing.T) *harness {
	t.Helper()
	t.Setenv("CARD_ENDPOINT", "")
	t.Setenv("OCR_ENDPOINT", "")
	t.Setenv("AI_BASE_URL", "")
	t.Setenv("LOCAL_OMR_ENABLED", "")
	a, e := New(context.Background(), Config{Dev: true, Username: "teacher", Password: "a-long-test-password", DataDir: t.TempDir()})
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Init(context.Background()); e != nil {
		t.Fatal(e)
	}
	return &harness{t: t, a: a, h: a.Handler()}
}
func (h *harness) request(method, path string, body any, status int) *httptest.ResponseRecorder {
	h.t.Helper()
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	r := httptest.NewRequest(method, "http://localhost"+path, &b)
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("Content-Type", "application/json")
	if h.cookie != nil {
		r.AddCookie(h.cookie)
	}
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	if w.Code != status {
		h.t.Fatalf("%s %s status %d expected %d: %s", method, path, w.Code, status, w.Body.String())
	}
	return w
}
func parse[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatal(e, w.Body.String())
	}
	return v
}
func (h *harness) login() {
	w := h.request("POST", "/api/v1/auth/login", map[string]string{"username": "teacher", "password": "a-long-test-password"}, 200)
	h.cookie = w.Result().Cookies()[0]
	if !h.cookie.HttpOnly || h.cookie.SameSite != http.SameSiteStrictMode {
		h.t.Fatal("unsafe cookie")
	}
}

// legacy fixture deliberately exercises existing a4-v1 releases after v2 becomes the default.
func (h *harness) exam() Exam {
	ex := h.studentExam()
	var legacy Exam
	if e := h.a.Store.With(context.Background(), func(tx *store.Tx) error {
		if e := tx.Get("exam", ex.ID, &legacy); e != nil {
			return e
		}
		qs := []vision.Question{}
		for _, q := range legacy.Questions {
			qs = append(qs, vision.Question{ID: q.ID, Number: q.Number, Type: q.Type, Options: q.Options})
		}
		m, e := vision.BuildManifest(qs)
		if e != nil {
			return e
		}
		legacy.Template = &m
		legacy.StudentNumberDigits = 0
		for i := range legacy.Sheets {
			legacy.Sheets[i].Token = ID()
		}
		if e = tx.Put("release", legacy.ReleaseID, legacy.ID, "", legacy); e != nil {
			return e
		}
		return tx.Put("exam", legacy.ID, "", "", legacy)
	}); e != nil {
		h.t.Fatal(e)
	}
	legacy.Sheets = nil
	return legacy
}
func (h *harness) studentExam() Exam {
	h.login()
	c := parse[Class](h.t, h.request("POST", "/api/v1/classes", map[string]string{"name": "Class"}, 201))
	h.request("POST", "/api/v1/classes/"+c.ID+"/students", map[string]any{"students": []map[string]string{{"number": "001", "name": "=student"}}}, 201)
	q := grading.Question{Number: "1", Type: "single_choice", Options: []string{"A", "B"}, Answer: []string{"A"}, ScoreUnits: 100}
	ex := parse[Exam](h.t, h.request("POST", "/api/v1/exams", examInput{Title: "exam", ClassID: c.ID, Questions: []grading.Question{q}}, 201))
	return parse[Exam](h.t, h.request("POST", "/api/v1/exams/"+ex.ID+"/publish", map[string]int{"revision": ex.Revision}, 200))
}
func TestAuthenticationOriginAndLogout(t *testing.T) {
	h := setup(t)
	h.request("GET", "/api/v1/classes", nil, 401)
	r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", strings.NewReader(`{"username":"teacher","password":"a-long-test-password"}`))
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("missing Origin accepted")
	}
	r.Header.Set("Origin", "http://evil.example")
	w = httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign Origin accepted")
	}
	h.request("POST", "/api/v1/auth/login", map[string]string{"username": "teacher", "password": "wrong"}, 401)
	h.login()
	h.request("GET", "/api/v1/me", nil, 200)
	h.request("POST", "/api/v1/auth/logout", map[string]string{}, 200)
	h.request("GET", "/api/v1/me", nil, 401)
}
func TestRosterAtomicAndFreeze(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	h.request("PATCH", "/api/v1/exams/"+ex.ID, examInput{Title: "changed", ClassID: ex.ClassID, Revision: ex.Revision, Questions: ex.Questions}, 409)
	h.request("POST", "/api/v1/classes/"+ex.ClassID+"/students", map[string]any{"students": []map[string]string{{"number": "002", "name": "new"}, {"number": "001", "name": "duplicate"}}}, 409)
	w := h.request("GET", "/api/v1/classes/"+ex.ClassID+"/students", nil, 200)
	var v struct{ Items []Student }
	json.Unmarshal(w.Body.Bytes(), &v)
	if len(v.Items) != 1 || v.Items[0].Number != "001" {
		t.Fatal("atomic leading-zero roster", v)
	}
	h.request("POST", "/api/v1/classes/"+ex.ClassID+"/students", map[string]any{"students": []map[string]string{{"number": "002", "name": "later"}}}, 201)
	var persisted Exam
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &persisted) })
	if len(persisted.Sheets) != 1 {
		t.Fatal("published roster not frozen")
	}
	h.request("GET", "/api/v1/exams/"+ex.ID+"/cards?student_id="+persisted.Sheets[0].StudentID, nil, 200)
	h.request("POST", "/api/v1/exams/"+ex.ID+"/parse", map[string]string{}, 503)
	h.request("POST", "/api/v1/exams/"+ex.ID+"/captures", map[string]string{}, 503)
}
func TestCommitUniqueConflictReview(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	var frozen Exam
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &frozen) })
	token := frozen.Sheets[0].Token
	q := ex.Questions[0].ID
	answer := []grading.Answer{{QuestionID: q, Selected: []string{"A"}, State: "uncertain"}}
	c := Capture{ID: ID(), ExamID: ex.ID, ClientID: ID()}
	e := h.a.Store.With(context.Background(), func(tx *store.Tx) error { return h.a.commitRecognition(tx, &c, frozen, token, answer) })
	if e != nil || c.Status != "needs_review" {
		t.Fatal(c, e)
	}
	duplicate := Capture{ID: ID(), ExamID: ex.ID}
	h.a.Store.With(context.Background(), func(tx *store.Tx) error { return h.a.commitRecognition(tx, &duplicate, frozen, token, answer) })
	if duplicate.Status != "duplicate" {
		t.Fatal(duplicate)
	}
	different := Capture{ID: ID(), ExamID: ex.ID}
	h.a.Store.With(context.Background(), func(tx *store.Tx) error {
		return h.a.commitRecognition(tx, &different, frozen, token, []grading.Answer{{QuestionID: q, Selected: []string{"B"}, State: "answered"}})
	})
	if different.Status != "conflict" {
		t.Fatal(different)
	}
	in := map[string]any{"revision": 1, "reason": "checked image", "answers": []map[string]any{{"question_id": q, "selected": []string{"A"}}}}
	result := parse[Result](t, h.request("POST", "/api/v1/results/"+c.SubmissionID+"/review", in, 200))
	if result.Score != 100 || result.Status != "committed" || result.Revision != 2 {
		t.Fatal(result)
	}
	h.request("POST", "/api/v1/results/"+c.SubmissionID+"/review", in, 409)
	w := h.request("GET", "/api/v1/exams/"+ex.ID+"/export.csv", nil, 200)
	if !strings.Contains(w.Body.String(), "'=student") {
		t.Fatal("CSV injection", w.Body.String())
	}
	h.request("GET", "/api/v1/exams/"+ex.ID+"/results", nil, 200)
}
func TestCloudCaptureIdempotency(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	var frozen Exam
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &frozen) })
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(5 << 20)
		if strings.Contains(r.FormValue("template"), "score_units") {
			t.Error("answer key sent to provider")
		}
		json.NewEncoder(w).Encode(map[string]any{"token": frozen.Sheets[0].Token, "answers": []map[string]any{{"question_id": ex.Questions[0].ID, "selected": []string{"A"}, "state": "answered", "confidence": 0.99}}})
	}))
	defer cloud.Close()
	t.Setenv("CARD_ENDPOINT", cloud.URL)
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.SetGray(x, y, color.Gray{255})
		}
	}
	var raw bytes.Buffer
	png.Encode(&raw, img)
	client := ID()
	post := func(data []byte, status int) Capture {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("image", "scan.png")
		part.Write(data)
		form.WriteField("client_capture_id", client)
		form.Close()
		r := httptest.NewRequest("POST", "http://localhost/api/v1/exams/"+ex.ID+"/captures", &body)
		r.Header.Set("Content-Type", form.FormDataContentType())
		r.Header.Set("Origin", "http://localhost")
		r.Header.Set("Idempotency-Key", client)
		r.AddCookie(h.cookie)
		w := httptest.NewRecorder()
		h.h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("capture %d: %s", w.Code, w.Body.String())
		}
		return parse[Capture](t, w)
	}
	first := post(raw.Bytes(), 200)
	if first.Status != "committed" || first.Score != 100 {
		t.Fatal(first)
	}
	second := post(raw.Bytes(), 200)
	if second.ID != first.ID || !second.Deduplicated {
		t.Fatal("idempotency", second)
	}
	img.SetGray(0, 0, color.Gray{0})
	raw.Reset()
	png.Encode(&raw, img)
	post(raw.Bytes(), 409)
}
func TestConcurrentUniqueEffectiveSubmission(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	var frozen Exam
	if e := h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &frozen) }); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := Capture{ID: ID(), ExamID: ex.ID}
			errs <- h.a.Store.With(context.Background(), func(tx *store.Tx) error {
				return h.a.commitRecognition(tx, &c, frozen, frozen.Sheets[0].Token, []grading.Answer{{QuestionID: ex.Questions[0].ID, Selected: []string{"A"}, State: "answered"}})
			})
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if e := h.a.Store.View(context.Background(), func(tx *store.Tx) error {
		rows, e := tx.List("result", ex.ID)
		if len(rows) != 1 {
			t.Errorf("got %d effective results", len(rows))
		}
		return e
	}); e != nil {
		t.Fatal(e)
	}
}
func TestPDFUploadAndUnconfiguredParse(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "paper.pdf")
	part.Write([]byte("%PDF-1.4\n%uploaded original\n%%EOF"))
	form.Close()
	r := httptest.NewRequest("POST", "http://localhost/api/v1/exams/"+ex.ID+"/documents", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.Header.Set("Origin", "http://localhost")
	r.AddCookie(h.cookie)
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	d := parse[Document](t, w)
	if d.StoragePath != "" || d.Status != "uploaded" {
		t.Fatal(d)
	}
	file := h.request("GET", "/api/v1/documents/"+d.ID+"/file", nil, 200)
	if !strings.HasPrefix(file.Body.String(), "%PDF-") {
		t.Fatal("original PDF lost")
	}
	h.request("POST", "/api/v1/exams/"+ex.ID+"/parse", map[string]string{"document_id": d.ID}, 503)
	entries, e := os.ReadDir(h.a.Config.DataDir)
	if e != nil || len(entries) != 1 {
		t.Fatal("private original missing", entries, e)
	}
}
func TestSessionRotationAndJSONStrictness(t *testing.T) {
	h := setup(t)
	h.login()
	old := h.cookie
	h.login()
	current := h.cookie
	h.cookie = old
	h.request("GET", "/api/v1/me", nil, 401)
	h.cookie = current
	h.request("POST", "/api/v1/classes", map[string]any{"name": "class", "unexpected": true}, 400)
	h.request("GET", "/api/v1/classes", nil, 200)
}
func TestProductionRequiresExplicitConfiguration(t *testing.T) {
	if _, e := New(context.Background(), Config{Dev: true, PublicOrigin: "http://remote.example", Username: "teacher", Password: "long-password-here", DataDir: t.TempDir()}); e == nil {
		t.Fatal("remote insecure dev cookies")
	}
	_, e := New(context.Background(), Config{Username: "teacher", Password: "long-password-here", DataDir: t.TempDir()})
	if e == nil {
		t.Fatal("silent production memory")
	}
	if safeCSV(" \t=1") != "' \t=1" {
		t.Fatal("csv leading whitespace")
	}
}

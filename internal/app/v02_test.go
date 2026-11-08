package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/store"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestV2StudentIdentityLeadingZerosAndReleaseQR(t *testing.T) {
	h := setup(t)
	ex := h.studentExam()
	var frozen Exam
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &frozen) })
	if frozen.StudentNumberDigits != 3 || !isStudentCard(frozen) {
		t.Fatal("new publish not v2", frozen.StudentNumberDigits)
	}
	answers := []grading.Answer{{QuestionID: ex.Questions[0].ID, Selected: []string{"A"}, State: "answered"}}
	for _, state := range []string{"blank", "uncertain", "invalid_multiple"} {
		c := Capture{ID: ID(), StudentNumber: "001", IdentityState: state}
		if e := h.a.Store.With(context.Background(), func(tx *store.Tx) error {
			return h.a.commitRecognition(tx, &c, frozen, frozen.Sheets[0].Token, answers)
		}); e != nil || c.Status != "needs_review" || c.SubmissionID != "" {
			t.Fatal(c, e)
		}
	}
	unknown := Capture{ID: ID(), StudentNumber: "999", IdentityState: "recognized"}
	h.a.Store.With(context.Background(), func(tx *store.Tx) error { return h.a.commitRecognition(tx, &unknown, frozen, "", answers) })
	if unknown.Status != "needs_review" {
		t.Fatal(unknown)
	}
	good := Capture{ID: ID(), StudentNumber: "001", IdentityState: "recognized"}
	if e := h.a.Store.With(context.Background(), func(tx *store.Tx) error { return h.a.commitRecognition(tx, &good, frozen, "", answers) }); e != nil || good.Status != "committed" || good.StudentNumber != "001" {
		t.Fatal(good, e)
	}
	h.request("GET", "/api/v1/exams/"+ex.ID+"/print-sheet?student_ids="+frozen.Sheets[0].StudentID, nil, 200)
}
func TestV2SharedReleaseTokenBindsNumberNotFirstStudent(t *testing.T) {
	h := setup(t)
	ex := h.studentExam()
	var frozen Exam
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &frozen) })
	shared := frozen.Sheets[0].Token
	frozen.Sheets = append(frozen.Sheets, Sheet{StudentID: ID(), StudentNumber: "002", StudentName: "second", Token: shared})
	h.a.Store.With(context.Background(), func(tx *store.Tx) error { return tx.Put("exam", ex.ID, "", "", frozen) })
	answers := []grading.Answer{{QuestionID: ex.Questions[0].ID, Selected: []string{"A"}, State: "answered"}}
	c := Capture{ID: ID(), ClientID: ID(), ExamID: ex.ID, StudentNumber: "002", IdentityState: "recognized", Revision: 1}
	if e := h.a.Store.With(context.Background(), func(tx *store.Tx) error { return h.a.commitRecognition(tx, &c, frozen, shared, answers) }); e != nil || c.Status != "committed" {
		t.Fatal(c, e)
	}
	var result Result
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("result", c.SubmissionID, &result) })
	if result.StudentID != frozen.Sheets[1].StudentID || result.StudentNumber != "002" {
		t.Fatal("release QR incorrectly selected first student", result)
	}
	unbound := Capture{ID: ID(), ClientID: ID(), ExamID: ex.ID, StudentNumber: "999", IdentityState: "recognized", Revision: 1}
	h.a.Store.With(context.Background(), func(tx *store.Tx) error {
		if e := h.a.commitRecognition(tx, &unbound, frozen, "wrong-release", answers); e != nil {
			return e
		}
		return tx.Put("capture", unbound.ID, ex.ID, unbound.ClientID, unbound)
	})
	h.request("GET", "/api/v1/exams/"+ex.ID+"/unbound-captures", nil, 200)
	body := map[string]any{"revision": unbound.Revision, "student_id": frozen.Sheets[0].StudentID, "reason": "teacher verified paper and roster"}
	bound := parse[Capture](t, h.request("POST", "/api/v1/captures/"+unbound.ID+"/bind", body, 200))
	if bound.Status != "committed" || bound.StudentNumber != "001" {
		t.Fatal(bound)
	}
	h.request("POST", "/api/v1/captures/"+unbound.ID+"/bind", body, 409)
}
func TestV2PublishRejectsMixedWidth(t *testing.T) {
	h := setup(t)
	h.login()
	c := parse[Class](t, h.request("POST", "/api/v1/classes", map[string]string{"name": "mixed"}, 201))
	h.request("POST", "/api/v1/classes/"+c.ID+"/students", map[string]any{"students": []map[string]string{{"number": "001", "name": "one"}, {"number": "02", "name": "two"}}}, 201)
	ex := parse[Exam](t, h.request("POST", "/api/v1/exams", examInput{Title: "mixed", ClassID: c.ID, Questions: []grading.Question{{Number: "1", Type: "single_choice", Options: []string{"A", "B"}, Answer: []string{"A"}, ScoreUnits: 100}}}, 201))
	h.request("POST", "/api/v1/exams/"+ex.ID+"/publish", map[string]int{"revision": 1}, 400)
}
func pngFixture(t *testing.T) []byte {
	var b bytes.Buffer
	if e := png.Encode(&b, image.NewGray(image.Rect(0, 0, 128, 128))); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestOllamaPreparedPagesPersistProgressAndResume(t *testing.T) {
	h := setup(t)
	published := h.studentExam()
	draft := parse[Exam](t, h.request("POST", "/api/v1/exams", examInput{Title: "source", ClassID: published.ClassID}, 201))
	var calls atomic.Int32
	failSecond := true
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			n := calls.Add(1)
			if n == 2 && failSecond {
				w.WriteHeader(502)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"done": true, "done_reason": "stop", "message": map[string]string{"content": "page text"}})
			return
		}
		if r.URL.Path == "/chat/completions" {
			json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": `{"questions":[{"number":"1","type":"single_choice","options":["A","B"],"answer":[],"score_units":null}],"warnings":[]}`}}}})
			return
		}
		t.Error("unexpected provider route", r.URL.Path)
	}))
	defer gateway.Close()
	t.Setenv("OCR_TYPE", "ollama")
	t.Setenv("OCR_MODEL", "ocr-test")
	t.Setenv("OCR_ENDPOINT", gateway.URL)
	t.Setenv("AI_BASE_URL", gateway.URL)
	t.Setenv("AI_MODEL", "text-test")
	t.Setenv("AI_API_KEY", "fixture-key")
	key, e := h.a.save([]byte("%PDF-1.4\n%%EOF"))
	if e != nil {
		t.Fatal(e)
	}
	d := Document{ID: ID(), ExamID: draft.ID, Status: "uploaded", ContentType: "application/pdf", StoragePath: key}
	h.a.Store.With(context.Background(), func(tx *store.Tx) error { return tx.Put("document", d.ID, d.ExamID, "", d) })
	h.request("POST", "/api/v1/exams/"+draft.ID+"/parse", map[string]string{"document_id": d.ID}, 409)
	uploadPage := func(n string, raw []byte, status int) {
		var b bytes.Buffer
		form := multipart.NewWriter(&b)
		p, _ := form.CreateFormFile("image", "page.png")
		p.Write(raw)
		form.WriteField("page_number", n)
		form.WriteField("total_pages", "2")
		form.Close()
		r := httptest.NewRequest("POST", "http://localhost/api/v1/exams/"+draft.ID+"/documents/"+d.ID+"/pages", &b)
		r.AddCookie(h.cookie)
		r.Header.Set("Origin", "http://localhost")
		r.Header.Set("Content-Type", form.FormDataContentType())
		w := httptest.NewRecorder()
		h.h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	raw := pngFixture(t)
	uploadPage("1", raw, 201)
	uploadPage("1", raw, 201)
	uploadPage("2", raw, 201)
	// Run persisted tasks directly, one page per call; no implicit retry of the failed second inference.
	start := func() string {
		var taskID string
		if e := h.a.Store.With(context.Background(), func(tx *store.Tx) error {
			if e := tx.Get("document", d.ID, &d); e != nil {
				return e
			}
			var err error
			taskID, err = h.a.queueDocumentParse(tx, &d, "ollama")
			return err
		}); e != nil {
			t.Fatal(e)
		}
		return taskID
	}
	run := func(id string) {
		if e := h.a.processDocumentTask(context.Background(), id, nil); e != nil {
			t.Fatal(e)
		}
	}
	// Disable automatic memory chaining for deterministic page-by-page execution by holding the shared slot.
	h.a.ocrSlot <- struct{}{}
	defer func() { <-h.a.ocrSlot }()
	id := start()
	run(id)
	var tasks []store.Record
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { tasks, _ = tx.List("document_task", d.ID); return nil })
	for _, r := range tasks {
		var task DocumentTask
		json.Unmarshal(r.Data, &task)
		if task.Status == "queued" {
			run(r.ID)
			break
		}
	}
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("document", d.ID, &d) })
	if d.Status != "failed" || d.CompletedPages != 1 || calls.Load() != 2 {
		t.Fatal("failed page progress", d, calls.Load())
	}
	failSecond = false
	id = start()
	run(id)
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { tasks, _ = tx.List("document_task", d.ID); return nil })
	for _, r := range tasks {
		var task DocumentTask
		json.Unmarshal(r.Data, &task)
		if task.Generation == d.ParseGeneration && task.Status == "queued" {
			run(r.ID)
		}
	}
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("document", d.ID, &d) })
	if d.Status != "parsed" || d.Progress != 100 || calls.Load() != 3 {
		t.Fatal("resume re-ran completed page or failed finalization", d, calls.Load())
	}
	if !strings.Contains(string(mustJSON(d.Artifact)), "requires_teacher_confirmation") {
		t.Fatal("missing artifact")
	}
}

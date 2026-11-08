package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/papergrade/papergrade/internal/store"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestBlockedMemoryOCRDoesNotRejectCaptureAdmission(t *testing.T) {
	h := setup(t)
	ex := h.studentExam()
	started := make(chan struct{})
	resume := make(chan struct{})
	var once sync.Once
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		<-resume
		w.WriteHeader(502)
	}))
	defer gateway.Close()
	t.Setenv("OCR_ENDPOINT", gateway.URL)
	t.Setenv("OCR_TYPE", "ollama")
	t.Setenv("OCR_MODEL", "ocr-test")
	raw := pngFixture(t)
	key, e := h.a.save(raw)
	if e != nil {
		t.Fatal(e)
	}
	d := Document{ID: ID(), ExamID: ex.ID, Status: "uploaded", ContentType: "image/png", StoragePath: key}
	var taskID string
	if e = h.a.Store.With(context.Background(), func(tx *store.Tx) error {
		if e := tx.Put("document", d.ID, d.ExamID, "", d); e != nil {
			return e
		}
		var err error
		taskID, err = h.a.queueDocumentParse(tx, &d, "ollama")
		return err
	}); e != nil {
		t.Fatal(e)
	}
	h.a.runMemoryDocumentTask(taskID)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("OCR did not start")
	}
	defer func() {
		close(resume)
		deadline := time.After(3 * time.Second)
		for {
			var current Document
			h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("document", d.ID, &current) })
			if current.Status == "failed" {
				return
			}
			select {
			case <-deadline:
				t.Error("OCR failure not persisted")
				return
			default:
				time.Sleep(time.Millisecond)
			}
		}
	}()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("image", "capture.png")
	part.Write(raw)
	form.WriteField("client_capture_id", "capture-during-ocr")
	form.Close()
	r := httptest.NewRequest("POST", "http://localhost/api/v1/exams/"+ex.ID+"/captures", &body)
	r.Header.Set("Origin", "http://localhost")
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.Header.Set("Idempotency-Key", "capture-during-ocr")
	r.AddCookie(h.cookie)
	w := httptest.NewRecorder()
	h.h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("capture blocked by unrelated OCR: %d %s", w.Code, w.Body.String())
	}
	var c Capture
	if e = json.Unmarshal(w.Body.Bytes(), &c); e != nil {
		t.Fatal(e)
	}
	if c.ID == "" || c.Status != "failed" {
		t.Fatal("capture should be accepted then honestly fail synthetic image quality", c)
	}
}

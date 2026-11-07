package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/papergrade/papergrade/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConfiguredParsingCreatesArtifactNotPublishedAnswers(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ocr":
			raw, _ := io.ReadAll(r.Body)
			if !bytes.HasPrefix(raw, []byte("%PDF-")) {
				t.Error("not original PDF")
			}
			json.NewEncoder(w).Encode(map[string]any{"text": "1. Choose A or B. Score not provided.", "pages": 1})
		case "/chat/completions":
			json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": `{"questions":[{"number":"1","type":"single_choice","options":["A","B"],"answer":[],"score_units":null,"source_page":1,"review_flags":[]}],"warnings":[]}`}}}})
		default:
			t.Error("unexpected gateway path", r.URL.Path)
		}
	}))
	defer gateway.Close()
	t.Setenv("OCR_ENDPOINT", gateway.URL+"/ocr")
	t.Setenv("AI_BASE_URL", gateway.URL)
	t.Setenv("AI_API_KEY", "test-provider-secret")
	t.Setenv("AI_MODEL", "test-model")
	key, e := h.a.save([]byte("%PDF-1.4\n%%EOF"))
	if e != nil {
		t.Fatal(e)
	}
	d := Document{ID: ID(), ExamID: ex.ID, Status: "uploaded", Filename: "paper.pdf", StoragePath: key, CreatedAt: time.Now()}
	if e = h.a.Store.With(context.Background(), func(tx *store.Tx) error { return tx.Put("document", d.ID, d.ExamID, "", d) }); e != nil {
		t.Fatal(e)
	}
	out := parse[Document](t, h.request("POST", "/api/v1/exams/"+ex.ID+"/parse", map[string]string{"document_id": d.ID}, 200))
	if out.Status != "parsed" {
		t.Fatal(out)
	}
	raw, e := json.Marshal(out.Artifact)
	if e != nil || !strings.Contains(string(raw), "requires_teacher_confirmation") || !strings.Contains(string(raw), `"score_units":null`) {
		t.Fatal(string(raw), e)
	}
	after := parse[Exam](t, h.request("GET", "/api/v1/exams/"+ex.ID, nil, 200))
	if after.Revision != ex.Revision || after.Questions[0].ScoreUnits != 100 || len(after.Questions[0].Answer) != 1 {
		t.Fatal("parser changed frozen exam", after)
	}
}

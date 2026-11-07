package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/provider"
	"github.com/papergrade/papergrade/internal/store"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionedRegradePreservesReleaseAndReview(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	var frozen Exam
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &frozen) })
	q := ex.Questions[0].ID
	c := Capture{ID: ID(), ExamID: ex.ID}
	if e := h.a.Store.With(context.Background(), func(tx *store.Tx) error {
		return h.a.commitRecognition(tx, &c, frozen, frozen.Sheets[0].Token, []grading.Answer{{QuestionID: q, Selected: []string{"B"}, State: "uncertain"}})
	}); e != nil {
		t.Fatal(e)
	}
	input := map[string]any{"revision": ex.Revision, "reason": "corrected teacher key", "answers": []map[string]any{{"question_id": q, "answer": []string{"B"}}}}
	w := h.request("POST", "/api/v1/exams/"+ex.ID+"/regrade", input, 200)
	var response struct {
		Exam           Exam
		UpdatedResults int `json:"updated_results"`
	}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response.Exam.Revision != ex.Revision+1 || response.Exam.GradingVersion == "" || response.Exam.ReleaseID != ex.ReleaseID || response.UpdatedResults != 1 {
		t.Fatal(w.Body.String())
	}
	var release Exam
	var result Result
	var history []store.Record
	if e := h.a.Store.View(context.Background(), func(tx *store.Tx) error {
		if e := tx.Get("release", ex.ReleaseID, &release); e != nil {
			return e
		}
		if e := tx.Get("result", c.SubmissionID, &result); e != nil {
			return e
		}
		var e error
		history, e = tx.List("result_history", result.ID)
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if release.Questions[0].Answer[0] != "A" || result.Status != "needs_review" || result.Revision != 2 || len(history) != 1 || result.GradingVersion != response.Exam.GradingVersion {
		t.Fatal(release, result, history)
	}
	h.request("POST", "/api/v1/exams/"+ex.ID+"/regrade", input, 409)
	h.request("GET", "/api/v1/exams/"+ex.ID+"/grading-history", nil, 200)
	input["revision"] = response.Exam.Revision
	input["answers"] = []map[string]any{{"question_id": "unknown", "answer": []string{"A"}}}
	h.request("POST", "/api/v1/exams/"+ex.ID+"/regrade", input, 400)
}
func TestRegradeConcurrentWithCloudRecognitionUsesNewestRules(t *testing.T) {
	h := setup(t)
	ex := h.exam()
	var frozen Exam
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Get("exam", ex.ID, &frozen) })
	started := make(chan struct{})
	resume := make(chan struct{})
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-resume
		json.NewEncoder(w).Encode(map[string]any{"token": frozen.Sheets[0].Token, "answers": []map[string]any{{"question_id": ex.Questions[0].ID, "selected": []string{"B"}, "state": "answered", "confidence": .99}}})
	}))
	defer cloud.Close()
	t.Setenv("CARD_ENDPOINT", cloud.URL)
	var raw bytes.Buffer
	png.Encode(&raw, image.NewGray(image.Rect(0, 0, 100, 100)))
	key, e := h.a.save(raw.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	capture := Capture{ID: ID(), ExamID: ex.ID, ClientID: ID(), Path: key, Status: "queued"}
	h.a.Store.With(context.Background(), func(tx *store.Tx) error { return tx.Put("capture", capture.ID, ex.ID, capture.ClientID, capture) })
	done := make(chan error, 1)
	go func() { done <- h.a.processCapture(context.Background(), capture.ID, nil) }()
	<-started
	input := map[string]any{"revision": ex.Revision, "reason": "teacher corrected key while capture processing", "answers": []map[string]any{{"question_id": ex.Questions[0].ID, "answer": []string{"B"}}}}
	h.request("POST", "/api/v1/exams/"+ex.ID+"/regrade", input, 200)
	close(resume)
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	var result Result
	h.a.Store.View(context.Background(), func(tx *store.Tx) error { return tx.Find("result", ex.ID, frozen.Sheets[0].StudentID, &result) })
	if result.Score != 100 || result.GradingVersion == ex.ReleaseID {
		t.Fatal("stale worker used old key", result)
	}
}
func TestExplicitAdoptionPreservesMissingFieldsAndRequiresOverwriteConfirmation(t *testing.T) {
	h := setup(t)
	published := h.exam()
	draft := parse[Exam](t, h.request("POST", "/api/v1/exams", examInput{Title: "draft", ClassID: published.ClassID, Questions: []grading.Question{}}, 201))
	d := Document{ID: ID(), ExamID: draft.ID, Status: "parsed", Artifact: map[string]any{"analysis": provider.Analysis{Questions: []provider.Question{{Number: "1", Type: "single_choice", Options: []string{"A", "B"}, Answer: []string{}}, {Number: "2", Type: "essay"}}}}}
	if e := h.a.Store.With(context.Background(), func(tx *store.Tx) error { return tx.Put("document", d.ID, d.ExamID, "", d) }); e != nil {
		t.Fatal(e)
	}
	path := "/api/v1/exams/" + draft.ID + "/documents/" + d.ID + "/adopt"
	w := h.request("POST", path, map[string]any{"revision": draft.Revision}, 200)
	var out struct {
		Exam     Exam
		Imported int
		Skipped  []map[string]string
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Imported != 1 || len(out.Skipped) != 1 || out.Exam.Questions[0].ScoreUnits != 0 || len(out.Exam.Questions[0].Answer) != 0 || len(out.Exam.Questions[0].ReviewFlags) == 0 {
		t.Fatal(w.Body.String())
	}
	h.request("POST", "/api/v1/exams/"+draft.ID+"/publish", map[string]int{"revision": out.Exam.Revision}, 400)
	h.request("POST", path, map[string]any{"revision": out.Exam.Revision}, 409)
	h.request("POST", path, map[string]any{"revision": out.Exam.Revision, "replace_existing": true}, 200)
	h.request("POST", "/api/v1/exams/"+published.ID+"/documents/"+d.ID+"/adopt", map[string]any{"revision": published.Revision}, 409)
}

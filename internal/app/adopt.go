package app

import (
	"encoding/json"
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/provider"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"strings"
	"time"
)

func (a *App) adopt(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision        int  `json:"revision"`
		ReplaceExisting bool `json:"replace_existing"`
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	var ex Exam
	skipped := []map[string]string{}
	warnings := []string{}
	imported := 0
	e := a.Store.With(r.Context(), func(t *store.Tx) error {
		if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
			return e
		}
		if ex.Status != "draft" {
			return conflict("cannot adopt into an immutable published exam")
		}
		if ex.Revision != in.Revision {
			return conflict("stale draft revision")
		}
		if len(ex.Questions) > 0 && !in.ReplaceExisting {
			return conflict("draft already contains questions; explicit replace_existing confirmation required")
		}
		var d Document
		if e := t.Get("document", r.PathValue("docid"), &d); e != nil {
			return e
		}
		if d.ExamID != ex.ID {
			return store.ErrNotFound
		}
		if d.Status != "parsed" {
			return conflict("only successfully parsed document can be adopted")
		}
		raw, e := json.Marshal(d.Artifact)
		if e != nil {
			return e
		}
		var artifact struct {
			Analysis *provider.Analysis `json:"analysis"`
		}
		if e = json.Unmarshal(raw, &artifact); e != nil || artifact.Analysis == nil {
			return bad("document has no analysis artifact")
		}
		qs := []grading.Question{}
		for _, q := range artifact.Analysis.Questions {
			if q.Type != "single_choice" && q.Type != "multiple_choice" && q.Type != "true_false" {
				skipped = append(skipped, map[string]string{"number": q.Number, "reason": "unsupported question type " + q.Type})
				continue
			}
			score := 0
			if q.ScoreUnits != nil {
				score = *q.ScoreUnits
			}
			flags := append([]string{}, q.ReviewFlags...)
			flags = append(flags, "teacher_confirmation_required")
			if score == 0 {
				flags = append(flags, "missing_score")
			}
			if len(q.Answer) == 0 {
				flags = append(flags, "missing_answer")
			}
			candidate := grading.Question{ID: ID(), Number: q.Number, Type: q.Type, Options: q.Options, Answer: q.Answer, ScoreUnits: score, ReviewFlags: flags, Section: strings.TrimSpace(q.Section)}
			if e = grading.Validate([]grading.Question{candidate}, false); e != nil {
				skipped = append(skipped, map[string]string{"number": q.Number, "reason": e.Error()})
				continue
			}
			qs = append(qs, candidate)
		}
		if len(qs) == 0 {
			return bad("no supported valid objective questions; draft left unchanged")
		}
		if e = grading.Validate(qs, false); e != nil {
			return bad(e.Error())
		}
		if e = t.Put("draft_history", ID(), ex.ID, "", map[string]any{"exam": ex, "action": "adopt_document", "document_id": d.ID, "at": time.Now().UTC(), "actor": "teacher"}); e != nil {
			return e
		}
		ex.Questions = qs
		ex.Revision++
		imported = len(qs)
		warnings = artifact.Analysis.Warnings
		return t.Put("exam", ex.ID, "", "", ex)
	})
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]any{"exam": ex, "imported": imported, "skipped": skipped, "warnings": warnings, "requires_teacher_confirmation": true})
}

package app

import (
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"time"
)

type GradingVersion struct {
	ID        string             `json:"id"`
	ExamID    string             `json:"exam_id"`
	ReleaseID string             `json:"release_id"`
	Previous  string             `json:"previous_version,omitempty"`
	Revision  int                `json:"revision"`
	Questions []grading.Question `json:"questions"`
	Reason    string             `json:"reason"`
	CreatedAt time.Time          `json:"created_at"`
}

func gradingVersion(ex Exam) string {
	if ex.GradingVersion != "" {
		return ex.GradingVersion
	}
	return ex.ReleaseID
}
func (a *App) regrade(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int `json:"revision"`
		Answers  []struct {
			QuestionID string   `json:"question_id"`
			Answer     []string `json:"answer"`
		} `json:"answers"`
		Reason string `json:"reason"`
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	if !bounded(in.Reason, 1000) {
		fail(w, bad("regrade reason required (<=1000 bytes)"))
		return
	}
	var ex Exam
	updated := 0
	e := a.Store.With(r.Context(), func(t *store.Tx) error {
		if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
			return e
		}
		if ex.Status != "published" {
			return conflict("publish before explicit regrading")
		}
		if ex.Revision != in.Revision {
			return conflict("stale exam revision")
		}
		if len(in.Answers) != len(ex.Questions) {
			return bad("provide every question answer exactly once")
		}
		by := map[string][]string{}
		for _, v := range in.Answers {
			if _, ok := by[v.QuestionID]; ok {
				return bad("duplicate question ID")
			}
			by[v.QuestionID] = v.Answer
		}
		old := ex
		changed := false
		qs := append([]grading.Question{}, ex.Questions...)
		for i := range qs {
			answer, ok := by[qs[i].ID]
			if !ok {
				return bad("unknown or missing question ID")
			}
			if !grading.Equal(qs[i].Answer, answer) {
				changed = true
			}
			qs[i].Answer = grading.Set(answer)
		}
		if !changed {
			return bad("answer key is unchanged")
		}
		if e := grading.Validate(qs, true); e != nil {
			return bad(e.Error())
		}
		now := time.Now().UTC()
		previous := gradingVersion(old)
		var prior GradingVersion
		if e := t.Get("grading_version", previous, &prior); e == store.ErrNotFound {
			if e = t.Put("grading_version", previous, ex.ID, "", GradingVersion{ID: previous, ExamID: ex.ID, ReleaseID: ex.ReleaseID, Revision: old.Revision, Questions: old.Questions, Reason: "original published rules", CreatedAt: old.CreatedAt}); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
		ex.Questions = qs
		ex.Revision++
		ex.GradingVersion = ID()
		version := GradingVersion{ID: ex.GradingVersion, ExamID: ex.ID, ReleaseID: ex.ReleaseID, Previous: previous, Revision: ex.Revision, Questions: qs, Reason: in.Reason, CreatedAt: now}
		if e := t.Put("grading_version", version.ID, ex.ID, "", version); e != nil {
			return e
		}
		results, e := list[Result](t, "result", ex.ID)
		if e != nil {
			return e
		}
		for _, result := range results {
			oldStatus := result.Status
			g, e := grading.Calculate(qs, result.Answers)
			if e != nil {
				return e
			}
			if e = t.Put("result_history", ID(), result.ID, "", map[string]any{"result": result, "reason": in.Reason, "at": now, "actor": "teacher", "action": "regrade", "grading_version": version.ID}); e != nil {
				return e
			}
			result.Score = g.Score
			result.Max = g.Max
			result.Revision++
			result.GradingVersion = version.ID
			result.Status = "committed"
			if g.Reviews > 0 || oldStatus == "needs_review" {
				result.Status = "needs_review"
			}
			if e = t.Put("result", result.ID, result.ExamID, result.StudentID, result); e != nil {
				return e
			}
			updated++
		}
		return t.Put("exam", ex.ID, "", "", ex)
	})
	if e != nil {
		fail(w, e)
		return
	}
	ex.Sheets = nil
	respond(w, 200, map[string]any{"exam": ex, "updated_results": updated, "grading_version": ex.GradingVersion})
}
func (a *App) gradingHistory(w http.ResponseWriter, r *http.Request) {
	var versions []GradingVersion
	e := a.Store.View(r.Context(), func(t *store.Tx) error {
		var ex Exam
		if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
			return e
		}
		var e error
		versions, e = list[GradingVersion](t, "grading_version", ex.ID)
		return e
	})
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]any{"items": versions})
}

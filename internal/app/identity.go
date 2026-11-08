package app

import (
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"time"
)

func (a *App) commitStudentRecognition(t *store.Tx, c *Capture, ex Exam, token string, answers []grading.Answer) error {
	c.Answers = answers
	c.Revision++
	unbound := func(msg string) error { c.Status = "needs_review"; c.Message = msg; return nil }
	if c.IdentityState != "recognized" {
		return unbound("student number is blank, multiple or uncertain; explicit identity review required")
	}
	var sheet *Sheet
	for i := range ex.Sheets {
		if ex.Sheets[i].StudentNumber == c.StudentNumber {
			sheet = &ex.Sheets[i]
			break
		}
	}
	if sheet == nil {
		return unbound("recognized student number is not in the frozen roster")
	}
	if token != "" {
		valid := false
		for _, s := range ex.Sheets {
			if s.Token == token {
				valid = true
				break
			}
		}
		if !valid {
			return unbound("QR does not belong to this release")
		}
	}
	if c.Candidate != "" && token != "" && c.Candidate != token {
		return unbound("candidate QR disagrees with server recognition")
	}
	return a.bindRecognized(t, c, ex, *sheet, answers)
}
func (a *App) bindRecognized(t *store.Tx, c *Capture, ex Exam, s Sheet, answers []grading.Answer) error {
	legacy := ex
	legacy.Template = nil
	legacy.Sheets = []Sheet{s}
	c.Candidate = ""
	if e := a.commitRecognition(t, c, legacy, s.Token, answers); e != nil {
		return e
	}
	c.StudentNumber = s.StudentNumber
	return nil
}
func (a *App) registerIdentityReview(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/exams/{id}/unbound-captures", func(w http.ResponseWriter, r *http.Request) {
		out := []Capture{}
		e := a.Store.View(r.Context(), func(t *store.Tx) error {
			var ex Exam
			if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
				return e
			}
			rows, e := list[Capture](t, "capture", ex.ID)
			if e != nil {
				return e
			}
			for _, c := range rows {
				if c.SubmissionID == "" && c.Status == "needs_review" {
					out = append(out, publicCapture(c))
				}
			}
			return nil
		})
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]any{"items": out})
	})
	m.HandleFunc("POST /api/v1/captures/{id}/bind", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision  int    `json:"revision"`
			StudentID string `json:"student_id"`
			Reason    string `json:"reason"`
		}
		if e := decode(r, &in); e != nil {
			fail(w, e)
			return
		}
		if !bounded(in.Reason, 1000) {
			fail(w, bad("identity review reason required"))
			return
		}
		var c Capture
		e := a.Store.With(r.Context(), func(t *store.Tx) error {
			if e := t.Get("capture", r.PathValue("id"), &c); e != nil {
				return e
			}
			if c.Revision != in.Revision || c.Status != "needs_review" || c.SubmissionID != "" {
				return conflict("capture is not an unbound review at this revision")
			}
			var ex Exam
			if e := t.Get("exam", c.ExamID, &ex); e != nil {
				return e
			}
			for _, s := range ex.Sheets {
				if s.StudentID == in.StudentID {
					before := c
					if e := a.bindRecognized(t, &c, ex, s, c.Answers); e != nil {
						return e
					}
					c.Revision++
					c.IdentityState = "recognized"
					if e := t.Put("capture_history", ID(), c.ID, "", map[string]any{"before": before, "reason": in.Reason, "at": time.Now().UTC(), "actor": "teacher", "action": "explicit_identity_binding"}); e != nil {
						return e
					}
					return t.Put("capture", c.ID, c.ExamID, c.ClientID, c)
				}
			}
			return bad("student not in frozen roster")
		})
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, publicCapture(c))
	})
}

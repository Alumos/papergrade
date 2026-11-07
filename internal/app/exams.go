package app

import (
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/store"
	"github.com/papergrade/papergrade/internal/vision"
	"net/http"
	"time"
)

type examInput struct {
	Title     string             `json:"title"`
	Subject   string             `json:"subject"`
	ClassID   string             `json:"class_id"`
	Questions []grading.Question `json:"questions"`
	Revision  int                `json:"revision"`
}

func (a *App) enrich(t *store.Tx, e *Exam) error {
	received, reviews, err := t.ResultCounts(e.ID)
	if err != nil {
		return err
	}
	e.ReceivedCount = received
	e.ReviewCount = reviews
	if e.Status == "draft" {
		var c Class
		if err = t.Get("class", e.ClassID, &c); err != nil {
			return err
		}
		e.StudentCount = c.StudentCount
	}
	return nil
}
func applyExam(t *store.Tx, e *Exam, in examInput) error {
	if !bounded(in.Title, 300) || len(in.Subject) > 100 {
		return bad("title required; title<=300 subject<=100 bytes")
	}
	var c Class
	if err := t.Get("class", in.ClassID, &c); err != nil {
		return err
	}
	known := map[string]bool{}
	for _, q := range e.Questions {
		known[q.ID] = true
	}
	for i := range in.Questions {
		q := &in.Questions[i]
		if q.ID == "" {
			q.ID = ID()
		} else if !known[q.ID] {
			return bad("unknown question ID")
		}
	}
	if err := grading.Validate(in.Questions, false); err != nil {
		return bad(err.Error())
	}
	if in.Questions == nil {
		in.Questions = []grading.Question{}
	}
	e.Title = in.Title
	e.Subject = in.Subject
	e.ClassID = in.ClassID
	e.Questions = in.Questions
	e.StudentCount = c.StudentCount
	return nil
}
func (a *App) registerExams(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/exams", func(w http.ResponseWriter, r *http.Request) {
		var out []Exam
		err := a.Store.View(r.Context(), func(t *store.Tx) error {
			var err error
			out, err = list[Exam](t, "exam", "")
			if err != nil {
				return err
			}
			for i := range out {
				if err = a.enrich(t, &out[i]); err != nil {
					return err
				}
				out[i].Sheets = nil
			}
			return nil
		})
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, map[string]any{"items": out})
	})
	m.HandleFunc("POST /api/v1/exams", func(w http.ResponseWriter, r *http.Request) {
		var in examInput
		if err := decode(r, &in); err != nil {
			fail(w, err)
			return
		}
		e := Exam{ID: ID(), Status: "draft", Revision: 1, CreatedAt: time.Now().UTC()}
		err := a.Store.With(r.Context(), func(t *store.Tx) error {
			if err := applyExam(t, &e, in); err != nil {
				return err
			}
			return t.Put("exam", e.ID, "", "", e)
		})
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 201, e)
	})
	m.HandleFunc("GET /api/v1/exams/{id}", func(w http.ResponseWriter, r *http.Request) {
		var e Exam
		err := a.Store.View(r.Context(), func(t *store.Tx) error {
			if err := t.Get("exam", r.PathValue("id"), &e); err != nil {
				return err
			}
			return a.enrich(t, &e)
		})
		if err != nil {
			fail(w, err)
			return
		}
		e.Sheets = nil
		respond(w, 200, e)
	})
	m.HandleFunc("PATCH /api/v1/exams/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in examInput
		if err := decode(r, &in); err != nil {
			fail(w, err)
			return
		}
		var e Exam
		err := a.Store.With(r.Context(), func(t *store.Tx) error {
			if err := t.Get("exam", r.PathValue("id"), &e); err != nil {
				return err
			}
			if e.Status != "draft" {
				return conflict("published release is immutable")
			}
			if in.Revision != e.Revision {
				return conflict("stale revision")
			}
			if err := applyExam(t, &e, in); err != nil {
				return err
			}
			e.Revision++
			return t.Put("exam", e.ID, "", "", e)
		})
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, e)
	})
	m.HandleFunc("POST /api/v1/exams/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int `json:"revision"`
		}
		if err := decode(r, &in); err != nil {
			fail(w, err)
			return
		}
		var e Exam
		err := a.Store.With(r.Context(), func(t *store.Tx) error {
			if err := t.Get("exam", r.PathValue("id"), &e); err != nil {
				return err
			}
			if e.Status != "draft" || e.Revision != in.Revision {
				return conflict("stale revision or already published")
			}
			if err := grading.Validate(e.Questions, true); err != nil {
				return bad(err.Error())
			}
			qs := []vision.Question{}
			for _, q := range e.Questions {
				qs = append(qs, vision.Question{ID: q.ID, Number: q.Number, Type: q.Type, Options: q.Options})
			}
			manifest, err := vision.BuildManifest(qs)
			if err != nil {
				return bad(err.Error())
			}
			students, err := list[Student](t, "student", e.ClassID)
			if err != nil {
				return err
			}
			if len(students) == 0 {
				return bad("roster required before publish")
			}
			e.Sheets = []Sheet{}
			for _, s := range students {
				e.Sheets = append(e.Sheets, Sheet{s.ID, s.Number, s.Name, ID()})
			}
			e.StudentCount = len(students)
			e.Template = &manifest
			e.ReleaseID = ID()
			e.Status = "published"
			e.Revision++
			if err = t.Put("release", e.ReleaseID, e.ID, "", e); err != nil {
				return err
			}
			return t.Put("exam", e.ID, "", "", e)
		})
		if err != nil {
			fail(w, err)
			return
		}
		e.Sheets = nil
		respond(w, 200, e)
	})
	m.HandleFunc("POST /api/v1/exams/{id}/regrade", a.regrade)
	m.HandleFunc("GET /api/v1/exams/{id}/grading-history", a.gradingHistory)
	m.HandleFunc("POST /api/v1/exams/{id}/documents/{docid}/adopt", a.adopt)
	m.HandleFunc("GET /api/v1/exams/{id}/template", func(w http.ResponseWriter, r *http.Request) {
		var e Exam
		err := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("exam", r.PathValue("id"), &e) })
		if err != nil {
			fail(w, err)
			return
		}
		if e.Template == nil {
			fail(w, conflict("publish first"))
			return
		}
		respond(w, 200, e.Template)
	})
	m.HandleFunc("GET /api/v1/exams/{id}/sheets", func(w http.ResponseWriter, r *http.Request) {
		var e Exam
		err := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("exam", r.PathValue("id"), &e) })
		if err != nil {
			fail(w, err)
			return
		}
		if e.Status != "published" {
			fail(w, conflict("publish first"))
			return
		}
		respond(w, 200, map[string]any{"items": e.Sheets})
	})
	m.HandleFunc("GET /api/v1/exams/{id}/cards", func(w http.ResponseWriter, r *http.Request) {
		var e Exam
		err := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("exam", r.PathValue("id"), &e) })
		if err != nil {
			fail(w, err)
			return
		}
		if e.Template == nil {
			fail(w, conflict("publish first"))
			return
		}
		for _, s := range e.Sheets {
			if s.StudentID == r.URL.Query().Get("student_id") {
				b, err := vision.RenderSVG(*e.Template, e.Title, s.StudentNumber+" "+s.StudentName, s.Token)
				if err != nil {
					fail(w, err)
					return
				}
				w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
				w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
				w.Write(b)
				return
			}
		}
		fail(w, store.ErrNotFound)
	})
	a.registerResults(m)
}

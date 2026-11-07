package app

import (
	"encoding/csv"
	"fmt"
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"sort"
	"strings"
	"time"
)

func summary(e Exam, rs []Result) map[string]any {
	review, total, committed := 0, 0, 0
	for _, r := range rs {
		if r.Status == "needs_review" {
			review++
		} else {
			total += r.Score
			committed++
		}
	}
	avg := float64(0)
	if committed > 0 {
		avg = float64(total) / float64(committed)
	}
	max := 0
	for _, q := range e.Questions {
		max += q.ScoreUnits
	}
	return map[string]any{"expected": e.StudentCount, "received": len(rs), "needs_review": review, "missing": e.StudentCount - len(rs), "average_units": avg, "max_score_units": max}
}
func questionStats(ex Exam, rs []Result) []map[string]any {
	out := []map[string]any{}
	for _, q := range ex.Questions {
		full, blank, uncertain, sum := 0, 0, 0, 0
		selected := map[string]int{}
		for _, v := range q.Options {
			selected[v] = 0
		}
		for _, r := range rs {
			for _, a := range r.Answers {
				if a.QuestionID != q.ID {
					continue
				}
				if a.State == "uncertain" {
					uncertain++
					continue
				}
				if a.State == "blank" {
					blank++
				}
				for _, v := range a.Selected {
					selected[v]++
				}
				g, e := grading.Calculate([]grading.Question{q}, []grading.Answer{a})
				if e == nil {
					sum += g.Score
					if g.Score == q.ScoreUnits {
						full++
					}
				}
			}
		}
		out = append(out, map[string]any{"question_id": q.ID, "number": q.Number, "full_score_count": full, "blank_count": blank, "uncertain_count": uncertain, "score_units_total": sum, "selected_counts": selected})
	}
	return out
}
func safeCSV(s string) string {
	trim := strings.TrimLeft(s, " \t\r\n")
	if trim != "" && strings.ContainsRune("=+-@", rune(trim[0])) {
		return "'" + s
	}
	return s
}
func (a *App) registerResults(m *http.ServeMux) {
	for _, path := range []string{"results", "reviews", "export.csv"} {
		path := path
		m.HandleFunc("GET /api/v1/exams/{id}/"+path, func(w http.ResponseWriter, r *http.Request) {
			var ex Exam
			var rs []Result
			e := a.Store.View(r.Context(), func(t *store.Tx) error {
				if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
					return e
				}
				var e error
				rs, e = list[Result](t, "result", ex.ID)
				return e
			})
			if e != nil {
				fail(w, e)
				return
			}
			sort.Slice(rs, func(i, j int) bool { return rs[i].StudentNumber < rs[j].StudentNumber })
			if path == "export.csv" {
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.Header().Set("Content-Disposition", "attachment; filename=results.csv")
				w.Write([]byte{0xef, 0xbb, 0xbf})
				c := csv.NewWriter(w)
				c.Write([]string{"student_number", "student_name", "status", "score_units", "max_score_units", "revision"})
				for _, s := range ex.Sheets {
					var result *Result
					for i := range rs {
						if rs[i].StudentID == s.StudentID {
							result = &rs[i]
							break
						}
					}
					if result == nil {
						c.Write([]string{safeCSV(s.StudentNumber), safeCSV(s.StudentName), "missing", "", "", ""})
					} else {
						c.Write([]string{safeCSV(result.StudentNumber), safeCSV(result.StudentName), result.Status, fmt.Sprint(result.Score), fmt.Sprint(result.Max), fmt.Sprint(result.Revision)})
					}
				}
				c.Flush()
				return
			}
			if path == "reviews" {
				out := []Result{}
				for _, v := range rs {
					if v.Status == "needs_review" {
						out = append(out, v)
					}
				}
				respond(w, 200, map[string]any{"items": out})
				return
			}
			respond(w, 200, map[string]any{"items": rs, "summary": summary(ex, rs), "question_stats": questionStats(ex, rs)})
		})
	}
	m.HandleFunc("POST /api/v1/results/{id}/review", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int `json:"revision"`
			Answers  []struct {
				QuestionID string   `json:"question_id"`
				Selected   []string `json:"selected"`
			} `json:"answers"`
			Reason string `json:"reason"`
		}
		if e := decode(r, &in); e != nil {
			fail(w, e)
			return
		}
		if !bounded(in.Reason, 1000) {
			fail(w, bad("review reason required (<=1000 bytes)"))
			return
		}
		var result Result
		e := a.Store.With(r.Context(), func(t *store.Tx) error {
			if e := t.Get("result", r.PathValue("id"), &result); e != nil {
				return e
			}
			if result.Revision != in.Revision {
				return conflict("stale answer revision")
			}
			var ex Exam
			if e := t.Get("exam", result.ExamID, &ex); e != nil {
				return e
			}
			if len(in.Answers) != len(ex.Questions) {
				return bad("review must include every question")
			}
			answers := []grading.Answer{}
			for _, v := range in.Answers {
				state := "answered"
				if len(v.Selected) == 0 {
					state = "blank"
				}
				for _, q := range ex.Questions {
					if q.ID == v.QuestionID && q.Type != "multiple_choice" && len(v.Selected) > 1 {
						state = "invalid_multiple"
					}
				}
				answers = append(answers, grading.Answer{QuestionID: v.QuestionID, Selected: v.Selected, State: state})
			}
			g, e := grading.Calculate(ex.Questions, answers)
			if e != nil {
				return bad(e.Error())
			}
			if g.Reviews > 0 {
				return bad("all questions must be resolved")
			}
			old := result
			if e = t.Put("result_history", ID(), result.ID, "", map[string]any{"result": old, "reason": in.Reason, "at": time.Now().UTC(), "actor": "teacher"}); e != nil {
				return e
			}
			result.Answers = answers
			result.Score = g.Score
			result.Max = g.Max
			result.Status = "committed"
			result.Revision++
			result.GradingVersion = gradingVersion(ex)
			return t.Put("result", result.ID, result.ExamID, result.StudentID, result)
		})
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, result)
	})
}

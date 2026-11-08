package app

import (
	"github.com/papergrade/papergrade/internal/store"
	"github.com/papergrade/papergrade/internal/vision"
	"net/http"
	"strings"
)

func (a *App) printSheet(w http.ResponseWriter, r *http.Request) {
	var ex Exam
	e := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("exam", r.PathValue("id"), &ex) })
	if e != nil {
		fail(w, e)
		return
	}
	if ex.Template == nil || (ex.Template.Version != vision.StudentVersion && ex.Template.Version != vision.EnglishVersion && ex.Template.Version != vision.EnglishV4Version) {
		fail(w, conflict("two-up printing requires a half-A4 student card release; legacy a4-v1 cards remain single-sheet"))
		return
	}
	ids := strings.Split(r.URL.Query().Get("student_ids"), ",")
	if len(ids) < 1 || len(ids) > 2 {
		fail(w, bad("select one or two students"))
		return
	}
	cards := []vision.PrintCard{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			fail(w, bad("duplicate selected student"))
			return
		}
		seen[id] = true
		found := false
		for _, s := range ex.Sheets {
			if s.StudentID == id {
				cards = append(cards, vision.PrintCard{StudentLabel: s.StudentNumber + " " + s.StudentName, Token: s.Token})
				found = true
				break
			}
		}
		if !found {
			fail(w, store.ErrNotFound)
			return
		}
	}
	raw, e := vision.RenderTwoUpSVG(*ex.Template, ex.Title, cards)
	if e != nil {
		fail(w, bad(e.Error()))
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.Write(raw)
}

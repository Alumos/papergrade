package app

import (
	"encoding/json"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"sort"
	"time"
)

func list[T any](t *store.Tx, kind, parent string) ([]T, error) {
	rows, e := t.List(kind, parent)
	if e != nil {
		return nil, e
	}
	out := []T{}
	for _, r := range rows {
		var x T
		if e = json.Unmarshal(r.Data, &x); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
func (a *App) registerClasses(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/classes", func(w http.ResponseWriter, r *http.Request) {
		var out []Class
		e := a.Store.View(r.Context(), func(t *store.Tx) error { var e error; out, e = list[Class](t, "class", ""); return e })
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]any{"items": out})
	})
	m.HandleFunc("POST /api/v1/classes", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if e := decode(r, &in); e != nil {
			fail(w, e)
			return
		}
		if !bounded(in.Name, 200) {
			fail(w, bad("class name required (<=200 bytes)"))
			return
		}
		c := Class{ID: ID(), Name: in.Name, CreatedAt: time.Now().UTC()}
		e := a.Store.With(r.Context(), func(t *store.Tx) error { return t.Put("class", c.ID, "", "", c) })
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 201, c)
	})
	m.HandleFunc("GET /api/v1/classes/{id}/students", func(w http.ResponseWriter, r *http.Request) {
		var out []Student
		e := a.Store.View(r.Context(), func(t *store.Tx) error {
			var c Class
			if e := t.Get("class", r.PathValue("id"), &c); e != nil {
				return e
			}
			var e error
			out, e = list[Student](t, "student", c.ID)
			sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
			return e
		})
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]any{"items": out})
	})
	m.HandleFunc("POST /api/v1/classes/{id}/students", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Students []struct {
				Number string `json:"number"`
				Name   string `json:"name"`
			} `json:"students"`
		}
		if e := decode(r, &in); e != nil {
			fail(w, e)
			return
		}
		if len(in.Students) < 1 || len(in.Students) > 500 {
			fail(w, bad("import 1–500 students"))
			return
		}
		e := a.Store.With(r.Context(), func(t *store.Tx) error {
			var c Class
			if e := t.Get("class", r.PathValue("id"), &c); e != nil {
				return e
			}
			existing, e := list[Student](t, "student", c.ID)
			if e != nil {
				return e
			}
			if len(existing)+len(in.Students) > 1000 {
				return bad("class limit 1000 students")
			}
			seen := map[string]bool{}
			for _, s := range existing {
				seen[s.Number] = true
			}
			for _, s := range in.Students {
				if !bounded(s.Number, 64) || !bounded(s.Name, 200) {
					return bad("number and name required")
				}
				if seen[s.Number] {
					return conflict("duplicate student number; import not applied")
				}
				seen[s.Number] = true
				v := Student{ID(), c.ID, s.Number, s.Name}
				if e = t.Put("student", v.ID, c.ID, v.Number, v); e != nil {
					return e
				}
			}
			c.StudentCount = len(seen)
			return t.Put("class", c.ID, "", "", c)
		})
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 201, map[string]int{"imported": len(in.Students)})
	})
}

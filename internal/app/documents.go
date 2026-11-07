package app

import (
	"bytes"
	"context"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"time"
)

func publicDocument(d Document) Document { d.StoragePath = ""; d.Path = ""; return d }
func (a *App) registerDocuments(m *http.ServeMux) {
	m.HandleFunc("POST /api/v1/exams/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		select {
		case a.heavy <- struct{}{}:
			defer func() { <-a.heavy }()
		default:
			fail(w, &APIError{429, "BUSY", "one upload at a time"})
			return
		}
		raw, name, e := upload(w, r, "file", 20<<20)
		if e != nil {
			fail(w, e)
			return
		}
		if !bytes.HasPrefix(raw, []byte("%PDF-")) {
			fail(w, bad("PDF signature required"))
			return
		}
		d := Document{ID: ID(), ExamID: r.PathValue("id"), Filename: name, Status: "uploaded", Size: len(raw), CreatedAt: time.Now().UTC()}
		key, e := a.save(raw)
		if e != nil {
			fail(w, e)
			return
		}
		d.StoragePath = key
		e = a.Store.With(r.Context(), func(t *store.Tx) error {
			var ex Exam
			if e := t.Get("exam", d.ExamID, &ex); e != nil {
				return e
			}
			docs, e := list[Document](t, "document", ex.ID)
			if e != nil {
				return e
			}
			if len(docs) >= 10 {
				return bad("maximum 10 documents per exam")
			}
			return t.Put("document", d.ID, d.ExamID, "", d)
		})
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 201, publicDocument(d))
	})
	m.HandleFunc("GET /api/v1/exams/{id}/documents", func(w http.ResponseWriter, r *http.Request) {
		var docs []Document
		e := a.Store.View(r.Context(), func(t *store.Tx) error {
			var ex Exam
			if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
				return e
			}
			var e error
			docs, e = list[Document](t, "document", ex.ID)
			return e
		})
		if e != nil {
			fail(w, e)
			return
		}
		for i := range docs {
			docs[i] = publicDocument(docs[i])
		}
		respond(w, 200, map[string]any{"items": docs})
	})
	m.HandleFunc("GET /api/v1/documents/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		var d Document
		e := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("document", r.PathValue("id"), &d) })
		if e != nil {
			fail(w, e)
			return
		}
		w.Header().Set("Content-Disposition", "inline; filename=paper.pdf")
		w.Header().Set("Content-Security-Policy", "sandbox")
		if e = a.serveFile(w, r, d.StoragePath, 20<<20, "application/pdf"); e != nil {
			fail(w, e)
		}
	})
	m.HandleFunc("POST /api/v1/exams/{id}/parse", func(w http.ResponseWriter, r *http.Request) {
		if a.Store.Mode() == "memory_dev" {
			select {
			case a.heavy <- struct{}{}:
				defer func() { <-a.heavy }()
			default:
				fail(w, &APIError{429, "BUSY", "one processing request at a time"})
				return
			}
		}
		caps := a.cloud().Capabilities()
		if !caps["ocr_configured"] || !caps["ai_configured"] {
			fail(w, &APIError{503, "PROVIDER_NOT_CONFIGURED", "OCR and AI providers are required; no parsing result fabricated"})
			return
		}
		var in struct {
			DocumentID string `json:"document_id"`
		}
		if e := decode(r, &in); e != nil {
			fail(w, e)
			return
		}
		var d Document
		e := a.Store.With(r.Context(), func(t *store.Tx) error {
			var ex Exam
			if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
				return e
			}
			docs, e := list[Document](t, "document", ex.ID)
			if e != nil {
				return e
			}
			for _, v := range docs {
				if v.ID == in.DocumentID || in.DocumentID == "" {
					d = v
					if in.DocumentID != "" {
						break
					}
				}
			}
			if d.ID == "" {
				return store.ErrNotFound
			}
			if d.Status == "queued" || d.Status == "processing" {
				return conflict("document already queued")
			}
			d.Status = "queued"
			if e = t.Put("document", d.ID, d.ExamID, "", d); e != nil {
				return e
			}
			return t.Enqueue(ID(), "parse", d.ID)
		})
		if e != nil {
			fail(w, e)
			return
		}
		if a.Store.Mode() == "memory_dev" {
			ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
			defer cancel()
			if e = a.processDocument(ctx, d.ID, nil); e != nil {
				fail(w, e)
				return
			}
			e = a.Store.With(r.Context(), func(t *store.Tx) error { return t.Get("document", d.ID, &d) })
			if e != nil {
				fail(w, e)
				return
			}
			respond(w, 200, publicDocument(d))
			return
		}
		respond(w, 202, publicDocument(d))
	})
}
func (a *App) processDocument(ctx context.Context, id string, job *store.Job) error {
	var d Document
	e := a.Store.With(ctx, func(t *store.Tx) error {
		if job != nil {
			if e := t.Fence(*job); e != nil {
				return e
			}
		}
		if e := t.Get("document", id, &d); e != nil {
			return e
		}
		d.Status = "processing"
		return t.Put("document", d.ID, d.ExamID, "", d)
	})
	if e != nil {
		return e
	}
	raw, e := a.load(d.StoragePath, 20<<20)
	var artifact any
	if e == nil {
		extraction, er := a.cloud().Extract(ctx, d.Filename, raw)
		e = er
		raw = nil
		if e == nil {
			analysis, er := a.cloud().Analyze(ctx, extraction.Text)
			e = er
			if e == nil {
				artifact = map[string]any{"extraction": extraction, "analysis": analysis, "requires_teacher_confirmation": true}
			}
		}
	}
	commitCtx, cancelCommit := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelCommit()
	return a.Store.With(commitCtx, func(t *store.Tx) error {
		if job != nil {
			if e := t.Fence(*job); e != nil {
				return e
			}
		}
		if er := t.Get("document", id, &d); er != nil {
			return er
		}
		d.Status = "parsed"
		d.Artifact = artifact
		if e != nil {
			d.Status = "failed"
			d.Artifact = map[string]any{"error": "provider parsing failed; no draft modified"}
		}
		if er := t.Put("document", d.ID, d.ExamID, "", d); er != nil {
			return er
		}
		if job != nil {
			return t.Done(*job)
		}
		return nil
	})
}

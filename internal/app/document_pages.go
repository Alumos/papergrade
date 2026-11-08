package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/papergrade/papergrade/internal/provider"
	"github.com/papergrade/papergrade/internal/store"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (a *App) registerDocumentPages(m *http.ServeMux) {
	m.HandleFunc("POST /api/v1/exams/{id}/documents/{docid}/pages", a.uploadDocumentPage)
	m.HandleFunc("GET /api/v1/exams/{id}/documents/{docid}/pages", a.getDocumentPages)
}
func (a *App) getDocumentPages(w http.ResponseWriter, r *http.Request) {
	pages := []DocumentPage{}
	e := a.Store.View(r.Context(), func(t *store.Tx) error {
		var d Document
		if e := t.Get("document", r.PathValue("docid"), &d); e != nil {
			return e
		}
		if d.ExamID != r.PathValue("id") {
			return store.ErrNotFound
		}
		var e error
		pages, e = list[DocumentPage](t, "document_page", d.ID)
		return e
	})
	if e != nil {
		fail(w, e)
		return
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].PageNumber < pages[j].PageNumber })
	for i := range pages {
		pages[i].StoragePath = ""
		pages[i].Hash = ""
	}
	respond(w, 200, map[string]any{"items": pages})
}
func (a *App) uploadDocumentPage(w http.ResponseWriter, r *http.Request) {
	select {
	case a.heavy <- struct{}{}:
		defer func() { <-a.heavy }()
	default:
		fail(w, &APIError{429, "BUSY", "one upload at a time"})
		return
	}
	raw, _, e := upload(w, r, "image", 4<<20)
	if e != nil {
		fail(w, e)
		return
	}
	content, e := provider.ImageType(raw)
	if e != nil {
		fail(w, bad(e.Error()))
		return
	}
	page, e := strconv.Atoi(r.FormValue("page_number"))
	total, e2 := strconv.Atoi(r.FormValue("total_pages"))
	if e != nil || e2 != nil || page < 1 || total < 1 || total > 30 || page > total {
		fail(w, bad("page_number and total_pages must be 1–30"))
		return
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	key, e := a.save(raw)
	if e != nil {
		fail(w, e)
		return
	}
	var d Document
	var p DocumentPage
	dedup := false
	e = a.Store.With(r.Context(), func(t *store.Tx) error {
		var ex Exam
		if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
			return e
		}
		if ex.Status != "draft" {
			return conflict("prepare pages only for a draft exam")
		}
		if e := t.Get("document", r.PathValue("docid"), &d); e != nil {
			return e
		}
		if d.ExamID != ex.ID {
			return store.ErrNotFound
		}
		if d.ContentType != "application/pdf" && d.ContentType != "" {
			return bad("page preparation is only required for PDF sources")
		}
		if d.Status != "uploaded" {
			return conflict("page evidence is immutable once parsing starts")
		}
		if d.TotalPages != 0 && d.TotalPages != total {
			return conflict("total_pages changed")
		}
		e := t.Find("document_page", d.ID, strconv.Itoa(page), &p)
		if e == nil {
			if p.Hash != hash {
				return conflict("page already uploaded with different image")
			}
			dedup = true
			return nil
		}
		if !errors.Is(e, store.ErrNotFound) {
			return e
		}
		p = DocumentPage{ID: ID(), DocumentID: d.ID, PageNumber: page, TotalPages: total, ContentType: content, StoragePath: key, Hash: hash, Status: "uploaded"}
		if e = t.Put("document_page", p.ID, d.ID, strconv.Itoa(page), p); e != nil {
			return e
		}
		d.TotalPages = total
		d.PreparedPages++
		d.PageErrors = []PageError{}
		return t.Put("document", d.ID, d.ExamID, "", d)
	})
	if e != nil {
		fail(w, e)
		return
	}
	p.StoragePath = ""
	p.Hash = ""
	respond(w, 201, map[string]any{"document": publicDocument(d), "page": p, "deduplicated": dedup})
}
func enqueueDocumentTask(t *store.Tx, d Document, phase string, page int) (string, error) {
	id := ID()
	task := DocumentTask{DocumentID: d.ID, PageNumber: page, Generation: d.ParseGeneration, Phase: phase, Status: "queued"}
	if e := t.Put("document_task", id, d.ID, "", task); e != nil {
		return "", e
	}
	if e := t.Enqueue(ID(), "document_task", id); e != nil {
		return "", e
	}
	return id, nil
}

// queueDocumentParse preserves completed pages on an explicit retry. No implicit replay of inference.
func (a *App) queueDocumentParse(t *store.Tx, d *Document, kind string) (string, error) {
	if d.Status == "queued" || d.Status == "processing" {
		return "", conflict("document already queued")
	}
	pages, e := list[DocumentPage](t, "document_page", d.ID)
	if e != nil {
		return "", e
	}
	if d.ContentType == "application/pdf" || d.ContentType == "" {
		if kind == "ollama" && (d.TotalPages < 1 || len(pages) != d.TotalPages) {
			return "", conflict("native Ollama requires every browser-rendered PDF page before parsing")
		}
		if kind != "ollama" && len(pages) == 0 {
			pages = []DocumentPage{{ID: ID(), DocumentID: d.ID, PageNumber: 1, TotalPages: 1, ContentType: "application/pdf", StoragePath: d.StoragePath, Status: "uploaded"}}
		}
	} else if len(pages) == 0 {
		pages = []DocumentPage{{ID: ID(), DocumentID: d.ID, PageNumber: 1, TotalPages: 1, ContentType: d.ContentType, StoragePath: d.StoragePath, Status: "uploaded"}}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].PageNumber < pages[j].PageNumber })
	if len(pages) == 0 {
		return "", bad("no OCR page evidence")
	}
	d.TotalPages = len(pages)
	d.PreparedPages = len(pages)
	d.CompletedPages = 0
	d.PageErrors = []PageError{}
	d.ParseGeneration++
	d.CurrentPage = 0
	first := 0
	for i, p := range pages {
		if p.PageNumber != i+1 {
			return "", conflict("prepared page sequence incomplete")
		}
		if p.Status == "completed" && p.Text != "" {
			d.CompletedPages++
		} else {
			p.Status = "uploaded"
			p.Error = ""
			if first == 0 {
				first = p.PageNumber
			}
		}
		if e = t.Put("document_page", p.ID, d.ID, strconv.Itoa(p.PageNumber), p); e != nil {
			return "", e
		}
	}
	d.Progress = d.CompletedPages * 90 / d.TotalPages
	d.Status = "queued"
	phase := "ocr"
	if first == 0 {
		phase = "ai"
	}
	task, e := enqueueDocumentTask(t, *d, phase, first)
	if e != nil {
		return "", e
	}
	if e = t.Put("document", d.ID, d.ExamID, "", *d); e != nil {
		return "", e
	}
	return task, nil
}
func (a *App) runMemoryDocumentTask(id string) {
	go func() {
		a.ocrSlot <- struct{}{}
		defer func() { <-a.ocrSlot }()
		ctx, cancel := context.WithTimeout(context.Background(), 310*time.Second)
		defer cancel()
		_ = a.processDocumentTask(ctx, id, nil)
	}()
}
func (a *App) processDocumentTask(ctx context.Context, id string, job *store.Job) error {
	var task DocumentTask
	var d Document
	var page DocumentPage
	interrupted := false
	e := a.Store.With(ctx, func(t *store.Tx) error {
		if job != nil {
			if e := t.Fence(*job); e != nil {
				return e
			}
		}
		if e := t.Get("document_task", id, &task); e != nil {
			return e
		}
		if e := t.Get("document", task.DocumentID, &d); e != nil {
			return e
		}
		if task.Generation != d.ParseGeneration {
			return conflict("stale document parse generation")
		}
		if task.Status == "completed" || task.Status == "failed" {
			return nil
		}
		if task.Status == "processing" {
			interrupted = true
		}
		task.Status = "processing"
		d.Status = "processing"
		d.CurrentPage = task.PageNumber
		if task.Phase == "ocr" {
			if e := t.Find("document_page", d.ID, strconv.Itoa(task.PageNumber), &page); e != nil {
				return e
			}
			page.Status = "processing"
			if e := t.Put("document_page", page.ID, d.ID, strconv.Itoa(page.PageNumber), page); e != nil {
				return e
			}
		}
		if e := t.Put("document", d.ID, d.ExamID, "", d); e != nil {
			return e
		}
		return t.Put("document_task", id, d.ID, "", task)
	})
	if e != nil {
		return e
	}
	if task.Status == "completed" || task.Status == "failed" {
		if job != nil {
			return a.Store.With(ctx, func(t *store.Tx) error { return t.Done(*job) })
		}
		return nil
	}
	var extraction provider.Extraction
	var analysis provider.Analysis
	callErr := error(nil)
	if interrupted {
		callErr = errors.New("interrupted model call requires explicit retry")
	} else {
		client, err := a.cloud(ctx)
		callErr = err
		if callErr == nil {
			if task.Phase == "ocr" {
				raw, err := a.load(page.StoragePath, 20<<20)
				callErr = err
				if err == nil {
					extraction, callErr = client.Extract(ctx, d.Filename, raw)
				}
			} else {
				var pages []DocumentPage
				callErr = a.Store.View(ctx, func(t *store.Tx) error {
					var e error
					pages, e = list[DocumentPage](t, "document_page", d.ID)
					return e
				})
				if callErr == nil {
					sort.Slice(pages, func(i, j int) bool { return pages[i].PageNumber < pages[j].PageNumber })
					var text strings.Builder
					for _, p := range pages {
						if p.Status != "completed" {
							callErr = errors.New("OCR page not complete")
							break
						}
						text.WriteString("\n\n--- Page ")
						text.WriteString(strconv.Itoa(p.PageNumber))
						text.WriteString(" ---\n")
						text.WriteString(p.Text)
						if text.Len() > 120000 {
							callErr = errors.New("aggregate OCR text exceeds limit")
							break
						}
					}
					if callErr == nil {
						extraction = provider.Extraction{Text: text.String(), Pages: len(pages), Source: "page_ocr"}
						analysis, callErr = client.Analyze(ctx, extraction.Text)
					}
				}
			}
		}
	}
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	next := ""
	e = a.Store.With(commitCtx, func(t *store.Tx) error {
		if job != nil {
			if e := t.Fence(*job); e != nil {
				return e
			}
		}
		var current Document
		if e := t.Get("document", d.ID, &current); e != nil {
			return e
		}
		if current.ParseGeneration != task.Generation {
			return conflict("stale document parse completion")
		}
		d = current
		task.Status = "completed"
		if callErr != nil {
			task.Status = "failed"
			d.Status = "failed"
			d.PageErrors = []PageError{{PageNumber: task.PageNumber, Message: "OCR/AI request failed or interrupted; explicit retry required"}}
			if task.Phase == "ocr" {
				page.Status = "failed"
				page.Error = d.PageErrors[0].Message
				if e := t.Put("document_page", page.ID, d.ID, strconv.Itoa(page.PageNumber), page); e != nil {
					return e
				}
			}
		} else if task.Phase == "ocr" {
			page.Status = "completed"
			page.Text = extraction.Text
			page.Error = ""
			if e := t.Put("document_page", page.ID, d.ID, strconv.Itoa(page.PageNumber), page); e != nil {
				return e
			}
			d.CompletedPages++
			d.Progress = d.CompletedPages * 90 / d.TotalPages
			phase := "ai"
			number := 0
			pages, e := list[DocumentPage](t, "document_page", d.ID)
			if e != nil {
				return e
			}
			sort.Slice(pages, func(i, j int) bool { return pages[i].PageNumber < pages[j].PageNumber })
			for _, p := range pages {
				if p.Status != "completed" {
					phase = "ocr"
					number = p.PageNumber
					break
				}
			}
			next, e = enqueueDocumentTask(t, d, phase, number)
			if e != nil {
				return e
			}
			d.Status = "queued"
		} else {
			d.Status = "parsed"
			d.Progress = 100
			d.CurrentPage = 0
			d.Artifact = map[string]any{"extraction": extraction, "analysis": analysis, "requires_teacher_confirmation": true}
		}
		if e := t.Put("document_task", id, d.ID, "", task); e != nil {
			return e
		}
		if e := t.Put("document", d.ID, d.ExamID, "", d); e != nil {
			return e
		}
		if job != nil {
			return t.Done(*job)
		}
		return nil
	})
	if e == nil && next != "" && a.Store.Mode() == "memory_dev" {
		a.runMemoryDocumentTask(next)
	}
	return e
}

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/papergrade/papergrade/internal/grading"
	"github.com/papergrade/papergrade/internal/provider"
	"github.com/papergrade/papergrade/internal/store"
	"github.com/papergrade/papergrade/internal/vision"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"time"
)

func (a *App) localOMR() bool { return a.Config.Dev && os.Getenv("LOCAL_OMR_ENABLED") == "true" }
func (a *App) capabilities(ctx context.Context) map[string]any {
	client, e := a.cloud(ctx)
	p := map[string]bool{}
	if e == nil {
		p = client.Capabilities()
	}
	mode := "disabled"
	if p["card_configured"] {
		mode = "cloud"
	} else if a.localOMR() {
		mode = "local_experimental"
	}
	return map[string]any{"ocr_configured": p["ocr_configured"], "ai_configured": p["ai_configured"], "card_recognition_configured": p["card_configured"], "recognition_mode": mode, "local_omr_enabled": a.localOMR(), "storage_mode": a.Store.Mode(), "regrade_available": true, "document_adopt_available": true, "max_capture_bytes": 4 << 20, "max_questions": 40, "score_units_per_point": 100, "provider_settings_available": e == nil, "provider_settings_configurable": a.providerCipher != nil}
}
func publicCapture(c Capture) Capture { c.Path = ""; c.Hash = ""; c.Candidate = ""; return c }
func (a *App) registerCaptures(m *http.ServeMux) {
	m.HandleFunc("GET /api/v1/captures/{id}", func(w http.ResponseWriter, r *http.Request) {
		var c Capture
		e := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("capture", r.PathValue("id"), &c) })
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, publicCapture(c))
	})
	m.HandleFunc("GET /api/v1/captures/{id}/image", func(w http.ResponseWriter, r *http.Request) {
		var c Capture
		e := a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("capture", r.PathValue("id"), &c) })
		if e != nil {
			fail(w, e)
			return
		}
		if e = a.serveFile(w, r, c.Path, 4<<20, ""); e != nil {
			fail(w, e)
		}
	})
	m.HandleFunc("POST /api/v1/exams/{id}/captures", func(w http.ResponseWriter, r *http.Request) {
		client, err := a.cloud(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		if !client.Capabilities()["card_configured"] && !a.localOMR() {
			fail(w, &APIError{503, "PROVIDER_NOT_CONFIGURED", "cloud card recognition is not configured; no capture accepted"})
			return
		}
		select {
		case a.heavy <- struct{}{}:
			defer func() { <-a.heavy }()
		default:
			w.Header().Set("Retry-After", "2")
			fail(w, &APIError{429, "BUSY", "one upload at a time"})
			return
		}
		raw, _, e := upload(w, r, "image", 4<<20)
		if e != nil {
			fail(w, e)
			return
		}
		clientID := r.FormValue("client_capture_id")
		if !bounded(clientID, 128) || r.Header.Get("Idempotency-Key") != clientID {
			fail(w, bad("Idempotency-Key must equal client_capture_id (<=128 bytes)"))
			return
		}
		cfg, format, e := image.DecodeConfig(bytes.NewReader(raw))
		if e != nil || (format != "jpeg" && format != "png") || cfg.Width < 100 || cfg.Height < 100 || cfg.Width > 8000000 || cfg.Height > 8000000 || int64(cfg.Width)*int64(cfg.Height) > 8000000 {
			fail(w, bad("JPEG/PNG required, 100px minimum dimensions and <=8MP"))
			return
		}
		h := sha256.Sum256(raw)
		hash := hex.EncodeToString(h[:])
		var c Capture
		isNew := false
		// Read-only fast path avoids writing another durable object on ordinary idempotent retries.
		var existing Capture
		e = a.Store.View(r.Context(), func(t *store.Tx) error {
			var ex Exam
			if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
				return e
			}
			if ex.Status != "published" {
				return conflict("publish before scanning")
			}
			return t.Find("capture", r.PathValue("id"), clientID, &existing)
		})
		if e == nil {
			if existing.Hash != hash {
				fail(w, conflict("idempotency key reused for different image"))
				return
			}
			existing.Deduplicated = true
			status := 200
			if existing.Status == "queued" || existing.Status == "processing" {
				status = 202
			}
			respond(w, status, publicCapture(existing))
			return
		}
		if !errors.Is(e, store.ErrNotFound) {
			fail(w, e)
			return
		}
		path, e := a.save(raw)
		if e != nil {
			fail(w, e)
			return
		}
		e = a.Store.With(r.Context(), func(t *store.Tx) error {
			var ex Exam
			if e := t.Get("exam", r.PathValue("id"), &ex); e != nil {
				return e
			}
			if ex.Status != "published" {
				return conflict("publish before scanning")
			}
			var old Capture
			lookupErr := t.Find("capture", ex.ID, clientID, &old)
			if lookupErr == nil {
				if old.Hash != hash {
					return conflict("idempotency key reused for different image")
				}
				c = old
				c.Deduplicated = true
				return nil
			}
			if !errors.Is(lookupErr, store.ErrNotFound) {
				return lookupErr
			}
			pending, e := t.PendingCaptures()
			if e != nil {
				return e
			}
			if pending >= 20 {
				return &APIError{429, "QUEUE_FULL", "capture queue full"}
			}
			c = Capture{ID: ID(), ClientID: clientID, ExamID: ex.ID, Status: "queued", Hash: hash, Path: path, Candidate: r.FormValue("candidate_sheet_token"), CreatedAt: time.Now().UTC()}
			if len(c.Candidate) > 512 {
				return bad("candidate token too long")
			}
			if e = t.Put("capture", c.ID, ex.ID, clientID, c); e != nil {
				return e
			}
			if e = t.Enqueue(ID(), "capture", c.ID); e != nil {
				return e
			}
			isNew = true
			return nil
		})
		if e != nil {
			fail(w, e)
			return
		}
		if a.Store.Mode() == "memory_dev" && isNew {
			ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
			defer cancel()
			if e = a.processCapture(ctx, c.ID, nil); e != nil {
				fail(w, e)
				return
			}
			e = a.Store.View(r.Context(), func(t *store.Tx) error { return t.Get("capture", c.ID, &c) })
			if e != nil {
				fail(w, e)
				return
			}
		}
		status := 202
		if c.Status != "queued" && c.Status != "processing" {
			status = 200
		}
		respond(w, status, publicCapture(c))
	})
}
func (a *App) recognize(ctx context.Context, raw []byte, ex Exam) (string, []grading.Answer, error) {
	out := []grading.Answer{}
	client, err := a.cloud(ctx)
	if err != nil {
		return "", nil, err
	}
	if client.Capabilities()["card_configured"] {
		qs := []provider.CardQuestion{}
		for _, q := range ex.Questions {
			qs = append(qs, provider.CardQuestion{ID: q.ID, Type: q.Type, Options: q.Options})
		}
		rec, e := client.RecognizeCard(ctx, raw, ex.Template, qs)
		if e != nil {
			return "", nil, e
		}
		for _, q := range rec.Answers {
			out = append(out, grading.Answer{QuestionID: q.QuestionID, Selected: q.Selected, State: q.State, Confidence: q.Confidence})
		}
		return rec.Token, out, nil
	}
	if !a.localOMR() {
		return "", nil, provider.ErrNotConfigured
	}
	img, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return "", nil, e
	}
	rec, e := vision.Recognize(img, *ex.Template)
	if e != nil {
		return "", nil, e
	}
	for _, q := range rec.Answers {
		out = append(out, grading.Answer{QuestionID: q.QuestionID, Selected: q.Selected, State: q.State, Confidence: q.Confidence})
	}
	return rec.Token, out, nil
}
func (a *App) processCapture(ctx context.Context, id string, job *store.Job) error {
	var c Capture
	var ex Exam
	e := a.Store.With(ctx, func(t *store.Tx) error {
		if job != nil {
			if e := t.Fence(*job); e != nil {
				return e
			}
		}
		if e := t.Get("capture", id, &c); e != nil {
			return e
		}
		if c.Status != "queued" && c.Status != "processing" {
			return nil
		}
		if e := t.Get("exam", c.ExamID, &ex); e != nil {
			return e
		}
		c.Status = "processing"
		return t.Put("capture", c.ID, c.ExamID, c.ClientID, c)
	})
	if e != nil {
		return e
	}
	if ex.ID == "" {
		if job != nil {
			return a.Store.With(ctx, func(t *store.Tx) error { return t.Done(*job) })
		}
		return nil
	}
	raw, e := a.load(c.Path, 4<<20)
	var token string
	var answers []grading.Answer
	if e == nil {
		token, answers, e = a.recognize(ctx, raw, ex)
	}
	recognitionErr := e
	commitCtx, cancelCommit := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelCommit()
	return a.Store.With(commitCtx, func(t *store.Tx) error {
		if job != nil {
			if e := t.Fence(*job); e != nil {
				return e
			}
		}
		if e := t.Get("capture", id, &c); e != nil {
			return e
		}
		if recognitionErr != nil {
			c.Status = "failed"
			c.Message = "recognition rejected or provider unavailable; rescan required"
		} else {
			var current Exam
			if e := t.Get("exam", ex.ID, &current); e != nil {
				return e
			}
			if current.ReleaseID != ex.ReleaseID {
				return conflict("capture release changed")
			}
			// Geometry is frozen, but answers may have an explicit newer grading version. Score under the current version inside this commit lock.
			if e := a.commitRecognition(t, &c, current, token, answers); e != nil {
				return e
			}
		}
		if e := t.Put("capture", c.ID, c.ExamID, c.ClientID, c); e != nil {
			return e
		}
		if job != nil {
			return t.Done(*job)
		}
		return nil
	})
}
func (a *App) commitRecognition(t *store.Tx, c *Capture, ex Exam, token string, answers []grading.Answer) error {
	var sheet *Sheet
	for i := range ex.Sheets {
		if ex.Sheets[i].Token == token {
			sheet = &ex.Sheets[i]
			break
		}
	}
	c.Answers = answers
	if sheet == nil {
		c.Status = "failed"
		c.Message = "QR does not match this immutable exam release"
		return nil
	}
	if c.Candidate != "" && c.Candidate != token {
		c.Status = "conflict"
		c.Message = "browser identity disagrees with independently recognized QR; no result changed"
		return nil
	}
	g, e := grading.Calculate(ex.Questions, answers)
	if e != nil {
		c.Status = "failed"
		c.Message = "invalid recognition answer contract"
		return nil
	}
	c.StudentDisplay = sheet.StudentNumber + " " + sheet.StudentName
	c.Score = g.Score
	c.Max = g.Max
	c.ReviewCount = g.Reviews
	var old Result
	lookupErr := t.Find("result", ex.ID, sheet.StudentID, &old)
	if lookupErr == nil {
		same := sameAnswers(old.Answers, answers)
		c.Status = "conflict"
		c.Message = "student already has a different effective submission; nothing overwritten"
		if same {
			c.Status = "duplicate"
			c.Deduplicated = true
			c.Message = "same effective answers already received"
		}
		c.SubmissionID = old.ID
		return nil
	}
	if !errors.Is(lookupErr, store.ErrNotFound) {
		return lookupErr
	}
	status := "committed"
	if g.Reviews > 0 {
		status = "needs_review"
	}
	r := Result{ID: ID(), ExamID: ex.ID, StudentID: sheet.StudentID, StudentNumber: sheet.StudentNumber, StudentName: sheet.StudentName, Status: status, Score: g.Score, Max: g.Max, Answers: answers, Revision: 1, CaptureID: c.ID, Engine: "objective-v1", ReleaseID: ex.ReleaseID, GradingVersion: gradingVersion(ex)}
	if e = t.Put("result", r.ID, ex.ID, r.StudentID, r); e != nil {
		return e
	}
	c.Status = status
	c.SubmissionID = r.ID
	return nil
}
func sameAnswers(a, b []grading.Answer) bool {
	if len(a) != len(b) {
		return false
	}
	by := map[string]grading.Answer{}
	for _, v := range a {
		by[v.QuestionID] = v
	}
	for _, v := range b {
		x, ok := by[v.QuestionID]
		if !ok || x.State != v.State || !grading.Equal(x.Selected, v.Selected) {
			return false
		}
	}
	return true
}

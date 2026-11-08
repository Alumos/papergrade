package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/papergrade/papergrade/internal/store"
	"log/slog"
	"time"
)

func (a *App) Worker(ctx context.Context) error {
	if a.Store.Mode() == "memory_dev" {
		return fmt.Errorf("worker requires PostgreSQL; memory development processes cannot share state")
	}
	delay := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}
		job, e := a.Store.Claim(ctx)
		if errors.Is(e, store.ErrNoJob) {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
			if delay < 4*time.Second {
				delay *= 2
			}
			continue
		}
		if e != nil {
			return e
		}
		delay = time.Second
		timeout := 100 * time.Second
		if job.Kind == "parse" {
			timeout = 180 * time.Second
		}
		if job.Kind == "document_task" {
			timeout = 310 * time.Second
		}
		jobCtx, cancel := context.WithTimeout(ctx, timeout)
		var er error
		if job.Attempts > 3 {
			er = fmt.Errorf("job exhausted retries after worker interruption")
		} else {
			switch job.Kind {
			case "capture":
				er = a.processCapture(jobCtx, job.EntityID, &job)
			case "document_task":
				er = a.processDocumentTask(jobCtx, job.EntityID, &job)
			case "parse":
				er = a.processDocument(jobCtx, job.EntityID, &job)
			default:
				er = fmt.Errorf("unknown job kind")
			}
		}
		cancel()
		if er != nil {
			slog.Error("job failed", "kind", job.Kind, "error", er)
			if e = a.Store.Retry(ctx, job, "worker processing failed"); e != nil {
				if errors.Is(e, store.ErrConflict) {
					continue
				}
				return e
			}
			if job.Attempts >= 3 {
				_ = a.Store.With(ctx, func(t *store.Tx) error {
					if e := t.DeadGeneration(job); e != nil {
						return e
					}
					if job.Kind == "capture" {
						var c Capture
						if e := t.Get("capture", job.EntityID, &c); e != nil {
							return e
						}
						c.Status = "failed"
						c.Message = "processing failed after bounded retries"
						return t.Put("capture", c.ID, c.ExamID, c.ClientID, c)
					}
					if job.Kind == "parse" {
						var d Document
						if e := t.Get("document", job.EntityID, &d); e != nil {
							return e
						}
						d.Status = "failed"
						return t.Put("document", d.ID, d.ExamID, "", d)
					}
					return nil
				})
			}
		}
	}
}

package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func sqliteStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s, e := Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "papergrade.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if s.Mode() != "sqlite" {
		t.Fatalf("mode %q", s.Mode())
	}
	return s
}

func TestSQLiteEntitiesRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := sqliteStore(t)
	type row struct {
		Name  string
		Count int
	}
	if e := s.With(ctx, func(tx *Tx) error {
		if e := tx.Put("class", "c1", "", "", row{"一班", 2}); e != nil {
			return e
		}
		if e := tx.Put("student", "s1", "c1", "1932", row{"张三", 1}); e != nil {
			return e
		}
		return tx.Put("student", "s2", "c1", "1933", row{"李四", 1})
	}); e != nil {
		t.Fatal(e)
	}
	var got row
	if e := s.View(ctx, func(tx *Tx) error { return tx.Get("class", "c1", &got) }); e != nil || got.Name != "一班" {
		t.Fatalf("%+v %v", got, e)
	}
	if e := s.View(ctx, func(tx *Tx) error {
		var v row
		return tx.Find("student", "c1", "1933", &v)
	}); e != nil {
		t.Fatal(e)
	}
	if e := s.View(ctx, func(tx *Tx) error { return tx.Get("class", "missing", &got) }); !errors.Is(e, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", e)
	}
	var listed []Record
	if e := s.View(ctx, func(tx *Tx) error {
		var e error
		listed, e = tx.List("student", "c1")
		return e
	}); e != nil || len(listed) != 2 {
		t.Fatalf("list %d %v", len(listed), e)
	}
	// Unique key within the same parent must conflict, not overwrite.
	if e := s.With(ctx, func(tx *Tx) error {
		return tx.Put("student", "s3", "c1", "1932", row{"重复", 1})
	}); !errors.Is(e, ErrConflict) {
		t.Fatalf("expected conflict, got %v", e)
	}
	if e := s.With(ctx, func(tx *Tx) error { return tx.Delete("student", "s2") }); e != nil {
		t.Fatal(e)
	}
	if e := s.View(ctx, func(tx *Tx) error { return tx.Get("student", "s2", &got) }); !errors.Is(e, ErrNotFound) {
		t.Fatalf("delete failed: %v", e)
	}
}

func TestSQLiteQueueLeaseFenceAndRetry(t *testing.T) {
	ctx := context.Background()
	s := sqliteStore(t)
	if e := s.With(ctx, func(tx *Tx) error {
		if e := tx.Put("capture", "cap1", "", "", map[string]string{"status": "queued"}); e != nil {
			return e
		}
		return tx.Enqueue("job1", "capture", "cap1")
	}); e != nil {
		t.Fatal(e)
	}
	job, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if job.Kind != "capture" || job.EntityID != "cap1" || job.Attempts != 1 {
		t.Fatalf("%+v", job)
	}
	if e = s.With(ctx, func(tx *Tx) error { return tx.Fence(job) }); e != nil {
		t.Fatalf("fence should pass: %v", e)
	}
	// A stale generation must not be able to commit.
	stale := job
	stale.Generation--
	if e = s.With(ctx, func(tx *Tx) error { return tx.Done(stale) }); !errors.Is(e, ErrConflict) {
		t.Fatalf("stale generation accepted: %v", e)
	}
	// Retry requeues with a delay, so the job is not due immediately.
	if e = s.Retry(ctx, job, "transient"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Claim(ctx); e == nil {
		t.Fatal("delayed job was handed out before its available_at")
	}
	if e = s.With(ctx, func(tx *Tx) error {
		return tx.Put("capture", "cap2", "", "", map[string]string{"status": "queued"})
	}); e != nil {
		t.Fatal(e)
	}
	if e = s.With(ctx, func(tx *Tx) error { return tx.Enqueue("job2", "capture", "cap2") }); e != nil {
		t.Fatal(e)
	}
	// job2 is due now, so it must be handed out before the delayed job1.
	next, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if next.EntityID != "cap2" {
		t.Fatalf("expected cap2, got %+v", next)
	}
	if e = s.With(ctx, func(tx *Tx) error { return tx.Done(next) }); e != nil {
		t.Fatal(e)
	}
}

// Three failed attempts must bury the job instead of retrying forever.
func TestSQLiteRetryLimitBuriesJob(t *testing.T) {
	ctx := context.Background()
	s := sqliteStore(t)
	if e := s.With(ctx, func(tx *Tx) error {
		if e := tx.Put("capture", "cap1", "", "", map[string]string{"status": "queued"}); e != nil {
			return e
		}
		return tx.Enqueue("job1", "capture", "cap1")
	}); e != nil {
		t.Fatal(e)
	}
	var last Job
	for i := 0; i < 3; i++ {
		job, e := s.Claim(ctx)
		if e != nil {
			t.Fatalf("claim %d: %v", i, e)
		}
		last = job
		if e = s.Retry(ctx, job, "boom"); e != nil {
			t.Fatalf("retry %d: %v", i, e)
		}
		// Clear the retry delay so the next claim is allowed immediately.
		if _, e = s.sq.ExecContext(ctx, "UPDATE jobs SET available_at=0 WHERE id=?", job.ID); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.With(ctx, func(tx *Tx) error { return tx.DeadGeneration(last) }); e != nil {
		t.Fatalf("job should be dead after three attempts: %v", e)
	}
	if _, e := s.Claim(ctx); e == nil {
		t.Fatal("dead job was handed out again")
	}
}

// A worker that dies mid-job leaves a lease behind. The next claim must bury the
// job and mark the entity failed so the UI does not spin forever.
func TestSQLiteExpiredLeaseIsSweptAndEntityFailed(t *testing.T) {
	ctx := context.Background()
	s := sqliteStore(t)
	if e := s.With(ctx, func(tx *Tx) error {
		if e := tx.Put("capture", "cap1", "", "", map[string]string{"status": "processing"}); e != nil {
			return e
		}
		return tx.Enqueue("job1", "capture", "cap1")
	}); e != nil {
		t.Fatal(e)
	}
	job, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.sq.ExecContext(ctx, "UPDATE jobs SET attempts=3, lease_until=? WHERE id=?", time.Now().Add(-time.Minute).UnixMilli(), job.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Claim(ctx); e == nil {
		t.Fatal("expired exhausted job was handed out")
	}
	var capture map[string]string
	if e = s.View(ctx, func(tx *Tx) error { return tx.Get("capture", "cap1", &capture) }); e != nil {
		t.Fatal(e)
	}
	if capture["status"] != "failed" {
		t.Fatalf("entity not failed after sweep: %+v", capture)
	}
}

func TestSQLiteCountsAndSessionCleanup(t *testing.T) {
	ctx := context.Background()
	s := sqliteStore(t)
	if e := s.With(ctx, func(tx *Tx) error {
		if e := tx.Put("result", "r1", "exam1", "", map[string]string{"status": "needs_review"}); e != nil {
			return e
		}
		if e := tx.Put("result", "r2", "exam1", "", map[string]string{"status": "graded"}); e != nil {
			return e
		}
		if e := tx.Put("capture", "c1", "", "", map[string]string{"status": "queued"}); e != nil {
			return e
		}
		return tx.Put("session", "s1", "", "", map[string]any{"expires": time.Now().Add(-time.Hour)})
	}); e != nil {
		t.Fatal(e)
	}
	var received, reviews int
	if e := s.View(ctx, func(tx *Tx) error {
		var e error
		received, reviews, e = tx.ResultCounts("exam1")
		return e
	}); e != nil || received != 2 || reviews != 1 {
		t.Fatalf("counts %d/%d %v", received, reviews, e)
	}
	var pending int
	if e := s.View(ctx, func(tx *Tx) error {
		var e error
		pending, e = tx.PendingCaptures()
		return e
	}); e != nil || pending != 1 {
		t.Fatalf("pending %d %v", pending, e)
	}
	if e := s.With(ctx, func(tx *Tx) error { return tx.CleanSessions(time.Now()) }); e != nil {
		t.Fatal(e)
	}
	var v any
	if e := s.View(ctx, func(tx *Tx) error { return tx.Get("session", "s1", &v) }); !errors.Is(e, ErrNotFound) {
		t.Fatalf("expired session survived: %v", e)
	}
}

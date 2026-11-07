package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"os"
	"testing"
	"time"
)

func exercise(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	id := "test-" + time.Now().Format("150405.000000000")
	defer s.With(ctx, func(tx *Tx) error { tx.Delete("test", id); tx.Delete("test", id+"2"); return nil })
	e := s.With(ctx, func(tx *Tx) error {
		if e := tx.Put("test", id, id, "001", map[string]string{"number": "001"}); e != nil {
			return e
		}
		return errors.New("rollback")
	})
	if e == nil {
		t.Fatal("expected error")
	}
	e = s.With(ctx, func(tx *Tx) error { var out map[string]string; return tx.Get("test", id, &out) })
	if !errors.Is(e, ErrNotFound) {
		t.Fatal("rollback failed", e)
	}
	if e = s.With(ctx, func(tx *Tx) error { return tx.Put("test", id, id, "001", map[string]string{"number": "001"}) }); e != nil {
		t.Fatal(e)
	}
	e = s.With(ctx, func(tx *Tx) error { return tx.Put("test", id+"2", id, "001", map[string]string{}) })
	if e == nil {
		t.Fatal("uniqueness not enforced")
	}
}
func TestMemoryAtomic(t *testing.T) { s, _ := Open(context.Background(), ""); exercise(t, s) }
func TestPostgresIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not configured: PostgreSQL integration NOT executed")
	}
	ctx := context.Background()
	s, e := Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	exercise(t, s)
	// A second independent pool must read the committed object, unlike a memory map.
	persistedID := "persist-" + time.Now().Format("150405.000000000")
	if e = s.With(ctx, func(tx *Tx) error { return tx.Put("test", persistedID, "", "", map[string]string{"value": "durable"}) }); e != nil {
		t.Fatal(e)
	}
	defer s.With(ctx, func(tx *Tx) error { return tx.Delete("test", persistedID) })
	other, e := Open(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	var out map[string]string
	e = other.View(ctx, func(tx *Tx) error { return tx.Get("test", persistedID, &out) })
	other.Close()
	if e != nil || out["value"] != "durable" {
		t.Fatal("independent connection persistence", out, e)
	}
	id := "job-" + time.Now().Format("150405.000000000")
	defer s.Pool.Exec(ctx, "DELETE FROM jobs WHERE id=$1", id)
	if e = s.With(ctx, func(tx *Tx) error { return tx.Enqueue(id, "capture", id) }); e != nil {
		t.Fatal(e)
	}
	j, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if j.ID != id {
		t.Fatal("requires clean isolated test DB; unexpected job", j)
	}
	_, e = s.Pool.Exec(ctx, "UPDATE jobs SET lease_until=now()-interval '1 second' WHERE id=$1", id)
	if e != nil {
		t.Fatal(e)
	}
	newJob, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if newJob.Generation <= j.Generation {
		t.Fatal("fence not monotonic")
	}
	if e = s.With(ctx, func(tx *Tx) error { return tx.Done(j) }); e == nil {
		t.Fatal("stale worker accepted")
	}
	if e = s.With(ctx, func(tx *Tx) error { return tx.Done(newJob) }); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Claim(ctx); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("done job reclaimed", e)
	}
	crashID := id + "-crash"
	defer s.Pool.Exec(ctx, "DELETE FROM jobs WHERE id=$1", crashID)
	defer s.With(ctx, func(tx *Tx) error { return tx.Delete("capture", crashID) })
	if e = s.With(ctx, func(tx *Tx) error {
		if e := tx.Put("capture", crashID, "test", "", map[string]string{"status": "queued"}); e != nil {
			return e
		}
		return tx.Enqueue(crashID, "capture", crashID)
	}); e != nil {
		t.Fatal(e)
	}
	var crashed Job
	for i := 1; i <= 3; i++ {
		crashed, e = s.Claim(ctx)
		if e != nil || crashed.ID != crashID || crashed.Attempts != i {
			t.Fatal("crash claim", crashed, e)
		}
		if _, e = s.Pool.Exec(ctx, "UPDATE jobs SET lease_until=now()-interval '1 second' WHERE id=$1", crashID); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 2; i++ {
		if _, e = s.Claim(ctx); !errors.Is(e, pgx.ErrNoRows) {
			t.Fatal("exhausted crash job reclaimed", e)
		}
	}
	var status string
	var attempts int
	if e = s.Pool.QueryRow(ctx, "SELECT status,attempts FROM jobs WHERE id=$1", crashID).Scan(&status, &attempts); e != nil || status != "dead" || attempts != 3 {
		t.Fatal("crash bound not durable", status, attempts, e)
	}
	var capture map[string]string
	if e = s.View(ctx, func(tx *Tx) error { return tx.Get("capture", crashID, &capture) }); e != nil || capture["status"] != "failed" {
		t.Fatal("crash capture not terminal", capture, e)
	}
	if e = s.With(ctx, func(tx *Tx) error { return tx.Done(crashed) }); e == nil {
		t.Fatal("exhausted crash generation committed")
	}
}

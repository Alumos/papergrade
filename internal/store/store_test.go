package store

import (
	"context"
	"errors"
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

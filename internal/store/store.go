// Package store persists independent JSON entity rows in a single SQLite file.
// The deployment target is a small VPS shared with other workloads, so there is
// no database server: one file, one writer, no network round trips.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	// ErrNoJob reports an empty queue; the worker loop uses it to idle.
	ErrNoJob = errors.New("no queued job")
)

type Record struct {
	Kind, ID, Parent, Key string
	Data                  json.RawMessage
}

type Store struct {
	sq *sql.DB
	mu sync.Mutex
	// mem backs the ephemeral development store when no path is configured.
	mem map[string]Record
}

type Tx struct {
	s   *Store
	ctx context.Context
	stx *sql.Tx
	mem map[string]Record
}

// Schema is created on every start. Every statement is IF NOT EXISTS, so this is
// the migration: no separate migrate container is needed for a single file.
const Schema = `
CREATE TABLE IF NOT EXISTS entities(
  kind TEXT NOT NULL,
  id TEXT NOT NULL,
  parent TEXT NOT NULL DEFAULT '',
  unique_key TEXT NOT NULL DEFAULT '',
  data TEXT NOT NULL,
  PRIMARY KEY(kind,id)
);
CREATE UNIQUE INDEX IF NOT EXISTS entities_unique ON entities(kind,parent,unique_key) WHERE unique_key<>'';
CREATE INDEX IF NOT EXISTS entities_parent ON entities(kind,parent);
CREATE TABLE IF NOT EXISTS jobs(
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  entity_id TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'queued',
  attempts INTEGER NOT NULL DEFAULT 0,
  available_at INTEGER NOT NULL,
  lease_until INTEGER,
  generation INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS jobs_available ON jobs(status,available_at);
`

// Open accepts "sqlite:<path>" for a durable file, or "" for ephemeral memory
// used only by DEV_MODE. PostgreSQL is no longer supported.
func Open(ctx context.Context, url string) (*Store, error) {
	s := &Store{mem: map[string]Record{}}
	if url == "" {
		return s, nil
	}
	path, ok := strings.CutPrefix(url, "sqlite:")
	if !ok {
		return nil, fmt.Errorf("unsupported store URL: use sqlite:<path>")
	}
	if path == "" {
		return nil, fmt.Errorf("sqlite path required")
	}
	if e := ensureDir(path); e != nil {
		return nil, fmt.Errorf("sqlite data directory: %w", e)
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, e := sql.Open("sqlite", dsn)
	if e != nil {
		return nil, e
	}
	// One connection serialises readers and writers. With a handful of teachers
	// this removes SQLITE_BUSY entirely instead of retrying around it.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	s.sq = db
	return s, nil
}

func (s *Store) Close() {
	if s.sq != nil {
		s.sq.Close()
	}
}

func (s *Store) Mode() string {
	if s.sq == nil {
		return "memory_dev"
	}
	return "sqlite"
}

// Ready reports whether the database file is usable.
func (s *Store) Ready(ctx context.Context) error {
	if s.sq == nil {
		return nil
	}
	var n int
	return s.sq.QueryRowContext(ctx, "SELECT 1").Scan(&n)
}

func (s *Store) Migrate(ctx context.Context) error {
	if s.sq == nil {
		return fmt.Errorf("SQLITE_PATH required for migrate")
	}
	_, e := s.sq.ExecContext(ctx, Schema)
	return e
}

func (s *Store) With(ctx context.Context, fn func(*Tx) error) error {
	if s.sq == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		m := map[string]Record{}
		for k, v := range s.mem {
			m[k] = v
		}
		t := &Tx{s: s, ctx: ctx, mem: m}
		if e := fn(t); e != nil {
			return e
		}
		s.mem = m
		return nil
	}
	tx, e := s.sq.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(&Tx{s: s, ctx: ctx, stx: tx}); e != nil {
		return e
	}
	return tx.Commit()
}

func (s *Store) View(ctx context.Context, fn func(*Tx) error) error {
	if s.sq == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return fn(&Tx{s: s, ctx: ctx, mem: s.mem})
	}
	tx, e := s.sq.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	return fn(&Tx{s: s, ctx: ctx, stx: tx})
}

func (t *Tx) Find(kind, parent, key string, out any) error {
	if t.stx != nil {
		return t.sqliteFind(kind, parent, key, out)
	}
	for _, r := range t.mem {
		if r.Kind == kind && r.Parent == parent && r.Key == key {
			return json.Unmarshal(r.Data, out)
		}
	}
	return ErrNotFound
}

func (t *Tx) ResultCounts(parent string) (received, reviews int, err error) {
	if t.stx != nil {
		return t.sqliteResultCounts(parent)
	}
	for _, r := range t.mem {
		if r.Kind != "result" || r.Parent != parent {
			continue
		}
		received++
		var v struct{ Status string }
		if e := json.Unmarshal(r.Data, &v); e != nil {
			return 0, 0, e
		}
		if v.Status == "needs_review" {
			reviews++
		}
	}
	return
}

func (t *Tx) PendingCaptures() (int, error) {
	if t.stx != nil {
		return t.sqlitePendingCaptures()
	}
	n := 0
	for _, r := range t.mem {
		if r.Kind != "capture" {
			continue
		}
		var c struct{ Status string }
		if e := json.Unmarshal(r.Data, &c); e != nil {
			return 0, e
		}
		if c.Status == "queued" || c.Status == "processing" {
			n++
		}
	}
	return n, nil
}

func (t *Tx) Get(kind, id string, out any) error {
	if t.stx != nil {
		return t.sqliteGet(kind, id, out)
	}
	r, ok := t.mem[kind+"/"+id]
	if !ok {
		return ErrNotFound
	}
	return json.Unmarshal(r.Data, out)
}

func (t *Tx) List(kind, parent string) ([]Record, error) {
	if t.stx != nil {
		return t.sqliteList(kind, parent)
	}
	out := []Record{}
	for _, r := range t.mem {
		if r.Kind == kind && (parent == "*" || r.Parent == parent) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (t *Tx) Put(kind, id, parent, key string, v any) error {
	if t.stx != nil {
		return t.sqlitePut(kind, id, parent, key, v)
	}
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	for _, r := range t.mem {
		if key != "" && r.Kind == kind && r.Parent == parent && r.Key == key && r.ID != id {
			return ErrConflict
		}
	}
	t.mem[kind+"/"+id] = Record{kind, id, parent, key, b}
	return nil
}

func (t *Tx) Delete(kind, id string) error {
	if t.stx != nil {
		return t.sqliteDelete(kind, id)
	}
	delete(t.mem, kind+"/"+id)
	return nil
}

func (t *Tx) Enqueue(id, kind, entity string) error {
	if t.stx != nil {
		return t.sqliteEnqueue(id, kind, entity)
	}
	return nil
}

type Job struct {
	ID, Kind, EntityID string
	Generation         int64
	Attempts           int
}

func (s *Store) Claim(ctx context.Context) (Job, error) {
	if s.sq == nil {
		return Job{}, fmt.Errorf("worker requires SQLITE_PATH")
	}
	return s.sqliteClaim(ctx)
}

func (t *Tx) Fence(j Job) error {
	if t.stx != nil {
		return t.sqliteFence(j)
	}
	return nil
}

func (t *Tx) Done(j Job) error {
	if t.stx != nil {
		return t.sqliteDone(j)
	}
	return nil
}

func (t *Tx) DeadGeneration(j Job) error {
	if t.stx != nil {
		return t.sqliteDeadGeneration(j)
	}
	return ErrNotFound
}

func (s *Store) Retry(ctx context.Context, j Job, msg string) error {
	if s.sq == nil {
		return nil
	}
	return s.sqliteRetry(ctx, j, msg)
}

func (t *Tx) CleanSessions(now time.Time) error {
	if t.stx != nil {
		return t.sqliteCleanSessions(now)
	}
	for k, r := range t.mem {
		if r.Kind != "session" {
			continue
		}
		var s struct{ Expires time.Time }
		if e := json.Unmarshal(r.Data, &s); e != nil {
			return e
		}
		if now.After(s.Expires) {
			delete(t.mem, k)
		}
	}
	return nil
}

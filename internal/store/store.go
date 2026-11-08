// Package store persists independent JSONB entity rows, with database uniqueness.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strconv"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

type Record struct {
	Kind, ID, Parent, Key string
	Data                  json.RawMessage
}
type Store struct {
	Pool *pgxpool.Pool
	mu   sync.Mutex
	mem  map[string]Record
}
type Tx struct {
	s   *Store
	ctx context.Context
	db  pgx.Tx
	mem map[string]Record
}

const Schema = `CREATE TABLE IF NOT EXISTS entities(kind text NOT NULL,id text NOT NULL,parent text NOT NULL DEFAULT '',unique_key text NOT NULL DEFAULT '',data jsonb NOT NULL,PRIMARY KEY(kind,id));
CREATE UNIQUE INDEX IF NOT EXISTS entities_unique ON entities(kind,parent,unique_key) WHERE unique_key<>'';
CREATE INDEX IF NOT EXISTS entities_parent ON entities(kind,parent);
CREATE INDEX IF NOT EXISTS captures_pending ON entities((data->>'status')) WHERE kind='capture' AND data->>'status' IN ('queued','processing');
CREATE TABLE IF NOT EXISTS jobs(id text PRIMARY KEY,kind text NOT NULL,entity_id text NOT NULL UNIQUE,status text NOT NULL DEFAULT 'queued',attempts integer NOT NULL DEFAULT 0,available_at timestamptz NOT NULL DEFAULT now(),lease_until timestamptz,generation bigint NOT NULL DEFAULT 0,last_error text NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS jobs_available ON jobs(status,available_at);`

func Open(ctx context.Context, url string) (*Store, error) {
	s := &Store{mem: map[string]Record{}}
	if url == "" {
		return s, nil
	}
	c, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, e
	}
	c.MaxConns = 3
	if n, e := strconv.Atoi(os.Getenv("DB_MAX_CONNS")); e == nil && n >= 1 && n <= 3 {
		c.MaxConns = int32(n)
	}
	c.MinConns = 0
	c.MaxConnIdleTime = 30 * time.Second
	c.MaxConnLifetime = 30 * time.Minute
	c.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	s.Pool, e = pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		return nil, e
	}
	if e = s.Pool.Ping(ctx); e != nil {
		s.Pool.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}
func (s *Store) Mode() string {
	if s.Pool == nil {
		return "memory_dev"
	}
	return "postgres"
}
func (s *Store) Migrate(ctx context.Context) error {
	if s.Pool == nil {
		return fmt.Errorf("DATABASE_URL required for migrate")
	}
	_, e := s.Pool.Exec(ctx, Schema)
	return e
}
func (t *Tx) CleanSessions(now time.Time) error {
	if t.db == nil {
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
	_, e := t.db.Exec(t.ctx, "DELETE FROM entities WHERE kind='session' AND (data->>'expires')::timestamptz<$1", now)
	return e
}
func (s *Store) Ready(ctx context.Context) error {
	if s.Pool == nil {
		return nil
	}
	var n int
	if e := s.Pool.QueryRow(ctx, "SELECT count(*) FROM entities WHERE false").Scan(&n); e != nil {
		return e
	}
	return s.Pool.QueryRow(ctx, "SELECT count(*) FROM jobs WHERE false").Scan(&n)
}
func (s *Store) With(ctx context.Context, fn func(*Tx) error) error {
	if s.Pool == nil {
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
	db, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer db.Rollback(ctx)
	// Single installation writer lock makes cross-row freeze/roster/result invariants atomic.
	if _, e = db.Exec(ctx, "SELECT pg_advisory_xact_lock(724791)"); e != nil {
		return e
	}
	t := &Tx{s: s, ctx: ctx, db: db}
	if e = fn(t); e != nil {
		return e
	}
	return db.Commit(ctx)
}

// View uses a repeatable-read snapshot and never acquires the installation writer lock.
func (s *Store) View(ctx context.Context, fn func(*Tx) error) error {
	if s.Pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return fn(&Tx{s: s, ctx: ctx, mem: s.mem})
	}
	db, e := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return e
	}
	defer db.Rollback(ctx)
	if e = fn(&Tx{s: s, ctx: ctx, db: db}); e != nil {
		return e
	}
	return db.Commit(ctx)
}
func (t *Tx) Find(kind, parent, key string, out any) error {
	if t.db == nil {
		for _, r := range t.mem {
			if r.Kind == kind && r.Parent == parent && r.Key == key {
				return json.Unmarshal(r.Data, out)
			}
		}
		return ErrNotFound
	}
	var b []byte
	e := t.db.QueryRow(t.ctx, "SELECT data FROM entities WHERE kind=$1 AND parent=$2 AND unique_key=$3", kind, parent, key).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}
func (t *Tx) ResultCounts(parent string) (received, reviews int, err error) {
	if t.db == nil {
		for _, r := range t.mem {
			if r.Kind == "result" && r.Parent == parent {
				received++
				var v struct{ Status string }
				if e := json.Unmarshal(r.Data, &v); e != nil {
					return 0, 0, e
				}
				if v.Status == "needs_review" {
					reviews++
				}
			}
		}
		return
	}
	err = t.db.QueryRow(t.ctx, "SELECT count(*),count(*) FILTER (WHERE data->>'status'='needs_review') FROM entities WHERE kind='result' AND parent=$1", parent).Scan(&received, &reviews)
	return
}
func (t *Tx) PendingCaptures() (int, error) {
	if t.db == nil {
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
	var n int
	e := t.db.QueryRow(t.ctx, "SELECT count(*) FROM entities WHERE kind='capture' AND data->>'status' IN ('queued','processing')").Scan(&n)
	return n, e
}
func (t *Tx) Get(kind, id string, out any) error {
	var b []byte
	if t.db == nil {
		r, ok := t.mem[kind+"/"+id]
		if !ok {
			return ErrNotFound
		}
		b = r.Data
	} else {
		e := t.db.QueryRow(t.ctx, "SELECT data FROM entities WHERE kind=$1 AND id=$2", kind, id).Scan(&b)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if e != nil {
			return e
		}
	}
	return json.Unmarshal(b, out)
}
func (t *Tx) List(kind, parent string) ([]Record, error) {
	a := []Record{}
	if t.db == nil {
		for _, r := range t.mem {
			if r.Kind == kind && (parent == "*" || r.Parent == parent) {
				a = append(a, r)
			}
		}
		return a, nil
	}
	rows, e := t.db.Query(t.ctx, "SELECT kind,id,parent,unique_key,data FROM entities WHERE kind=$1 AND ($2='*' OR parent=$2) ORDER BY id", kind, parent)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var r Record
		if e = rows.Scan(&r.Kind, &r.ID, &r.Parent, &r.Key, &r.Data); e != nil {
			return nil, e
		}
		a = append(a, r)
	}
	return a, rows.Err()
}
func (t *Tx) Put(kind, id, parent, key string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if t.db == nil {
		for _, r := range t.mem {
			if key != "" && r.Kind == kind && r.Parent == parent && r.Key == key && r.ID != id {
				return ErrConflict
			}
		}
		t.mem[kind+"/"+id] = Record{kind, id, parent, key, b}
		return nil
	}
	_, e = t.db.Exec(t.ctx, "INSERT INTO entities(kind,id,parent,unique_key,data) VALUES($1,$2,$3,$4,$5) ON CONFLICT(kind,id) DO UPDATE SET parent=excluded.parent,unique_key=excluded.unique_key,data=excluded.data", kind, id, parent, key, b)
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		return ErrConflict
	}
	return e
}
func (t *Tx) Delete(kind, id string) error {
	if t.db == nil {
		delete(t.mem, kind+"/"+id)
		return nil
	}
	_, e := t.db.Exec(t.ctx, "DELETE FROM entities WHERE kind=$1 AND id=$2", kind, id)
	return e
}
func (t *Tx) Enqueue(id, kind, entity string) error {
	if t.db == nil {
		return nil
	}
	_, e := t.db.Exec(t.ctx, "INSERT INTO jobs(id,kind,entity_id) VALUES($1,$2,$3) ON CONFLICT(entity_id) DO UPDATE SET status='queued',available_at=now(),attempts=0 WHERE jobs.status IN ('done','dead')", id, kind, entity)
	return e
}

type Job struct {
	ID, Kind, EntityID string
	Generation         int64
	Attempts           int
}

func (s *Store) Claim(ctx context.Context) (Job, error) {
	var j Job
	if s.Pool == nil {
		return j, fmt.Errorf("worker requires DATABASE_URL")
	}
	e := s.Pool.QueryRow(ctx, `WITH exhausted AS (UPDATE jobs SET status='dead',generation=generation+1,lease_until=NULL,last_error='retry limit reached after worker interruption' WHERE attempts>=3 AND ((status='processing' AND lease_until<now()) OR status='queued') RETURNING entity_id,kind), failed_entities AS (UPDATE entities e SET data=jsonb_set(jsonb_set(e.data,'{status}','"failed"'::jsonb),'{message}','"processing stopped after bounded retries"'::jsonb) FROM exhausted x WHERE e.id=x.entity_id AND e.kind=CASE WHEN x.kind='parse' THEN 'document' ELSE x.kind END AND e.data->>'status' IN ('queued','processing') RETURNING e.id), candidate AS (SELECT id FROM jobs WHERE attempts<3 AND ((status='queued' AND available_at<=now()) OR (status='processing' AND lease_until<now())) ORDER BY CASE WHEN kind='capture' THEN 0 ELSE 1 END,available_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE jobs j SET status='processing',attempts=attempts+1,generation=generation+1,lease_until=now()+interval '360 seconds' FROM candidate c WHERE j.id=c.id RETURNING j.id,j.kind,j.entity_id,j.generation,j.attempts`).Scan(&j.ID, &j.Kind, &j.EntityID, &j.Generation, &j.Attempts)
	return j, e
}
func (t *Tx) Fence(j Job) error {
	if t.db == nil {
		return nil
	}
	var generation int64
	var until time.Time
	e := t.db.QueryRow(t.ctx, "SELECT generation,lease_until FROM jobs WHERE id=$1 AND status='processing' FOR UPDATE", j.ID).Scan(&generation, &until)
	if e != nil {
		return e
	}
	if generation != j.Generation || time.Now().After(until) {
		return ErrConflict
	}
	return nil
}
func (t *Tx) Done(j Job) error {
	if t.db == nil {
		return nil
	}
	if e := t.Fence(j); e != nil {
		return e
	}
	_, e := t.db.Exec(t.ctx, "UPDATE jobs SET status='done',lease_until=NULL WHERE id=$1 AND generation=$2", j.ID, j.Generation)
	return e
}
func (t *Tx) DeadGeneration(j Job) error {
	var n int
	return t.db.QueryRow(t.ctx, "SELECT 1 FROM jobs WHERE id=$1 AND generation=$2 AND status='dead' FOR UPDATE", j.ID, j.Generation).Scan(&n)
}
func (s *Store) Retry(ctx context.Context, j Job, msg string) error {
	tag, e := s.Pool.Exec(ctx, "UPDATE jobs SET status=CASE WHEN attempts>=3 THEN 'dead' ELSE 'queued' END,available_at=now()+interval '10 seconds',lease_until=NULL,last_error=$3 WHERE id=$1 AND generation=$2 AND status='processing' AND lease_until>now()", j.ID, j.Generation, msg)
	if e == nil && tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return e
}

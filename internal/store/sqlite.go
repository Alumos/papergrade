package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ensureDir creates the directory that will hold the database file.
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o750)
}

func (t *Tx) sqliteFind(kind, parent, key string, out any) error {
	var b string
	e := t.stx.QueryRowContext(t.ctx, "SELECT data FROM entities WHERE kind=? AND parent=? AND unique_key=?", kind, parent, key).Scan(&b)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	return json.Unmarshal([]byte(b), out)
}

func (t *Tx) sqliteGet(kind, id string, out any) error {
	var b string
	e := t.stx.QueryRowContext(t.ctx, "SELECT data FROM entities WHERE kind=? AND id=?", kind, id).Scan(&b)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	return json.Unmarshal([]byte(b), out)
}

func (t *Tx) sqliteList(kind, parent string) ([]Record, error) {
	rows, e := t.stx.QueryContext(t.ctx, "SELECT kind,id,parent,unique_key,data FROM entities WHERE kind=? AND (?='*' OR parent=?) ORDER BY id", kind, parent, parent)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var r Record
		var data string
		if e = rows.Scan(&r.Kind, &r.ID, &r.Parent, &r.Key, &data); e != nil {
			return nil, e
		}
		r.Data = json.RawMessage(data)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (t *Tx) sqlitePut(kind, id, parent, key string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = t.stx.ExecContext(t.ctx, "INSERT INTO entities(kind,id,parent,unique_key,data) VALUES(?,?,?,?,?) ON CONFLICT(kind,id) DO UPDATE SET parent=excluded.parent,unique_key=excluded.unique_key,data=excluded.data", kind, id, parent, key, string(b))
	if e != nil && strings.Contains(e.Error(), "UNIQUE constraint failed") {
		return ErrConflict
	}
	return e
}

func (t *Tx) sqliteDelete(kind, id string) error {
	_, e := t.stx.ExecContext(t.ctx, "DELETE FROM entities WHERE kind=? AND id=?", kind, id)
	return e
}

func (t *Tx) sqliteResultCounts(parent string) (int, int, error) {
	var received, reviews int
	e := t.stx.QueryRowContext(t.ctx, "SELECT count(*), coalesce(sum(CASE WHEN json_extract(data,'$.status')='needs_review' THEN 1 ELSE 0 END),0) FROM entities WHERE kind='result' AND parent=?", parent).Scan(&received, &reviews)
	return received, reviews, e
}

func (t *Tx) sqlitePendingCaptures() (int, error) {
	var n int
	e := t.stx.QueryRowContext(t.ctx, "SELECT count(*) FROM entities WHERE kind='capture' AND json_extract(data,'$.status') IN ('queued','processing')").Scan(&n)
	return n, e
}

// sqliteCleanSessions expires sessions in Go: the expiry is an RFC3339 string
// inside the JSON document, so parsing avoids dialect-specific string compares.
func (t *Tx) sqliteCleanSessions(now time.Time) error {
	rows, e := t.stx.QueryContext(t.ctx, "SELECT id,data FROM entities WHERE kind='session'")
	if e != nil {
		return e
	}
	expired := []string{}
	for rows.Next() {
		var id, data string
		if e = rows.Scan(&id, &data); e != nil {
			rows.Close()
			return e
		}
		var s struct{ Expires time.Time }
		if e = json.Unmarshal([]byte(data), &s); e != nil {
			rows.Close()
			return e
		}
		if now.After(s.Expires) {
			expired = append(expired, id)
		}
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return e
	}
	rows.Close()
	for _, id := range expired {
		if _, e = t.stx.ExecContext(t.ctx, "DELETE FROM entities WHERE kind='session' AND id=?", id); e != nil {
			return e
		}
	}
	return nil
}

func (t *Tx) sqliteEnqueue(id, kind, entity string) error {
	now := time.Now().UnixMilli()
	_, e := t.stx.ExecContext(t.ctx, "INSERT INTO jobs(id,kind,entity_id,available_at) VALUES(?,?,?,?) ON CONFLICT(entity_id) DO UPDATE SET status='queued',available_at=excluded.available_at,attempts=0 WHERE jobs.status IN ('done','dead')", id, kind, entity, now)
	return e
}

func (t *Tx) sqliteFence(j Job) error {
	var generation int64
	var until sql.NullInt64
	e := t.stx.QueryRowContext(t.ctx, "SELECT generation,lease_until FROM jobs WHERE id=? AND status='processing'", j.ID).Scan(&generation, &until)
	if e != nil {
		return e
	}
	if generation != j.Generation || !until.Valid || time.Now().After(time.UnixMilli(until.Int64)) {
		return ErrConflict
	}
	return nil
}

func (t *Tx) sqliteDone(j Job) error {
	if e := t.sqliteFence(j); e != nil {
		return e
	}
	_, e := t.stx.ExecContext(t.ctx, "UPDATE jobs SET status='done',lease_until=NULL WHERE id=? AND generation=?", j.ID, j.Generation)
	return e
}

func (t *Tx) sqliteDeadGeneration(j Job) error {
	var n int
	e := t.stx.QueryRowContext(t.ctx, "SELECT 1 FROM jobs WHERE id=? AND generation=? AND status='dead'", j.ID, j.Generation).Scan(&n)
	return e
}

// sqliteClaim takes the next due job. SQLite serialises writers, so the whole
// sweep-select-update sequence is atomic inside one transaction without the
// PostgreSQL SKIP LOCKED machinery.
func (s *Store) sqliteClaim(ctx context.Context) (Job, error) {
	var j Job
	tx, e := s.sq.BeginTx(ctx, nil)
	if e != nil {
		return j, e
	}
	defer tx.Rollback()
	now := time.Now()
	nowMS := now.UnixMilli()

	rows, e := tx.QueryContext(ctx, "SELECT id,entity_id,kind FROM jobs WHERE attempts>=3 AND ((status='processing' AND lease_until IS NOT NULL AND lease_until<?) OR status='queued')", nowMS)
	if e != nil {
		return j, e
	}
	type deadJob struct{ id, entity, kind string }
	buried := []deadJob{}
	for rows.Next() {
		var d deadJob
		if e = rows.Scan(&d.id, &d.entity, &d.kind); e != nil {
			rows.Close()
			return j, e
		}
		buried = append(buried, d)
	}
	rows.Close()
	for _, d := range buried {
		if _, e = tx.ExecContext(ctx, "UPDATE jobs SET status='dead',generation=generation+1,lease_until=NULL,last_error=? WHERE id=?", "retry limit reached after worker interruption", d.id); e != nil {
			return j, e
		}
		entityKind := d.kind
		if d.kind == "parse" {
			entityKind = "document"
		}
		if _, e = tx.ExecContext(ctx, "UPDATE entities SET data=json_set(json_set(data,'$.status','failed'),'$.message','processing stopped after bounded retries') WHERE id=? AND kind=? AND json_extract(data,'$.status') IN ('queued','processing')", d.entity, entityKind); e != nil {
			return j, e
		}
	}

	e = tx.QueryRowContext(ctx, "SELECT id,kind,entity_id,generation,attempts FROM jobs WHERE attempts<3 AND ((status='queued' AND available_at<=?) OR (status='processing' AND lease_until IS NOT NULL AND lease_until<?)) ORDER BY CASE WHEN kind='capture' THEN 0 ELSE 1 END, available_at LIMIT 1", nowMS, nowMS).Scan(&j.ID, &j.Kind, &j.EntityID, &j.Generation, &j.Attempts)
	if errors.Is(e, sql.ErrNoRows) {
		// Commit the sweep above, otherwise buried jobs would be rolled back.
		if e = tx.Commit(); e != nil {
			return Job{}, e
		}
		return Job{}, ErrNoJob
	}
	if e != nil {
		return j, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE jobs SET status='processing',attempts=attempts+1,generation=generation+1,lease_until=? WHERE id=? AND status IN ('queued','processing')", now.Add(360*time.Second).UnixMilli(), j.ID); e != nil {
		return j, e
	}
	if e = tx.Commit(); e != nil {
		return j, e
	}
	j.Generation++
	j.Attempts++
	return j, nil
}

func (s *Store) sqliteRetry(ctx context.Context, j Job, msg string) error {
	now := time.Now()
	res, e := s.sq.ExecContext(ctx, "UPDATE jobs SET status=CASE WHEN attempts>=3 THEN 'dead' ELSE 'queued' END,available_at=?,lease_until=NULL,last_error=? WHERE id=? AND generation=? AND status='processing' AND lease_until>?", now.Add(10*time.Second).UnixMilli(), msg, j.ID, j.Generation, now.UnixMilli())
	if e != nil {
		return e
	}
	if n, e := res.RowsAffected(); e == nil && n == 0 {
		return ErrConflict
	}
	return nil
}

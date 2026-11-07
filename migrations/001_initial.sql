-- Applied by the migrate command; embedded counterpart internal/store.Schema.
CREATE TABLE IF NOT EXISTS entities(kind text NOT NULL,id text NOT NULL,parent text NOT NULL DEFAULT '',unique_key text NOT NULL DEFAULT '',data jsonb NOT NULL,PRIMARY KEY(kind,id));
CREATE UNIQUE INDEX IF NOT EXISTS entities_unique ON entities(kind,parent,unique_key) WHERE unique_key<>'';
CREATE INDEX IF NOT EXISTS entities_parent ON entities(kind,parent);
CREATE INDEX IF NOT EXISTS captures_pending ON entities((data->>'status')) WHERE kind='capture' AND data->>'status' IN ('queued','processing');
CREATE TABLE IF NOT EXISTS jobs(id text PRIMARY KEY,kind text NOT NULL,entity_id text NOT NULL UNIQUE,status text NOT NULL DEFAULT 'queued',attempts integer NOT NULL DEFAULT 0,available_at timestamptz NOT NULL DEFAULT now(),lease_until timestamptz,generation bigint NOT NULL DEFAULT 0,last_error text NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS jobs_available ON jobs(status,available_at);

CREATE TABLE audit_log (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT REFERENCES projects(id) ON DELETE RESTRICT,
    actor_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    action TEXT NOT NULL CHECK (btrim(action) <> ''),
    entity_type TEXT NOT NULL CHECK (btrim(entity_type) <> ''),
    entity_id TEXT NOT NULL CHECK (btrim(entity_id) <> ''),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    request_id UUID,
    details JSONB NOT NULL DEFAULT '{}'::JSONB CHECK (jsonb_typeof(details) = 'object')
);

CREATE INDEX audit_log_project_idx ON audit_log (project_id, occurred_at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id, occurred_at DESC);
CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id, occurred_at DESC);

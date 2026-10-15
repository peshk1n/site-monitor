CREATE TABLE incidents (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL,
    monitor_id INTEGER NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
    first_observed_at TIMESTAMPTZ NOT NULL,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    resolution TEXT CHECK (resolution IN ('recovered', 'archived', 'reconfigured', 'manual')),
    summary TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (monitor_id, project_id) REFERENCES monitors(id, project_id) ON DELETE RESTRICT,
    UNIQUE (id, monitor_id, project_id),
    CHECK (opened_at >= first_observed_at),
    CHECK (resolved_at IS NULL OR resolved_at >= opened_at),
    CHECK ((resolved_at IS NULL) = (resolution IS NULL))
);

CREATE UNIQUE INDEX incidents_open_monitor_idx ON incidents (monitor_id) WHERE resolved_at IS NULL;
CREATE INDEX incidents_monitor_history_idx ON incidents (monitor_id, opened_at DESC);
CREATE INDEX incidents_project_history_idx ON incidents (project_id, opened_at DESC);

CREATE TABLE monitor_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id BIGINT NOT NULL,
    monitor_id INTEGER NOT NULL,
    incident_id BIGINT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('incident_opened', 'severity_changed', 'incident_resolved')),
    severity TEXT NOT NULL CHECK (severity IN ('warning', 'critical')),
    config_version BIGINT NOT NULL CHECK (config_version > 0),
    deduplication_key TEXT NOT NULL UNIQUE CHECK (btrim(deduplication_key) <> ''),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    payload JSONB NOT NULL DEFAULT '{}'::JSONB CHECK (jsonb_typeof(payload) = 'object'),
    FOREIGN KEY (incident_id, monitor_id, project_id)
        REFERENCES incidents(id, monitor_id, project_id) ON DELETE RESTRICT,
    UNIQUE (id, project_id)
);

CREATE INDEX monitor_events_incident_idx ON monitor_events (incident_id, occurred_at);
CREATE INDEX monitor_events_project_idx ON monitor_events (project_id, occurred_at DESC);

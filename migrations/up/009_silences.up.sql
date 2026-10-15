CREATE TABLE silences (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('mute', 'maintenance')),
    scope TEXT NOT NULL CHECK (scope IN ('all', 'projects', 'monitors')),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL CHECK (btrim(reason) <> ''),
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cancelled_at TIMESTAMPTZ,
    cancelled_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (id, scope),
    CHECK (ends_at > starts_at),
    CHECK ((cancelled_at IS NULL) = (cancelled_by IS NULL)),
    CHECK (cancelled_at IS NULL OR cancelled_at >= created_at)
);

CREATE INDEX silences_window_idx ON silences (ends_at, starts_at) WHERE cancelled_at IS NULL;

CREATE TABLE silence_projects (
    silence_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    scope TEXT NOT NULL DEFAULT 'projects' CHECK (scope = 'projects'),
    PRIMARY KEY (silence_id, project_id),
    FOREIGN KEY (silence_id, scope) REFERENCES silences(id, scope) ON DELETE CASCADE
);

CREATE INDEX silence_projects_project_idx ON silence_projects (project_id);

CREATE TABLE silence_monitors (
    silence_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    monitor_id INTEGER NOT NULL,
    scope TEXT NOT NULL DEFAULT 'monitors' CHECK (scope = 'monitors'),
    PRIMARY KEY (silence_id, monitor_id),
    FOREIGN KEY (silence_id, scope) REFERENCES silences(id, scope) ON DELETE CASCADE,
    FOREIGN KEY (monitor_id, project_id) REFERENCES monitors(id, project_id) ON DELETE RESTRICT
);

CREATE INDEX silence_monitors_monitor_idx ON silence_monitors (monitor_id);

CREATE INDEX silence_monitors_project_idx ON silence_monitors (project_id);

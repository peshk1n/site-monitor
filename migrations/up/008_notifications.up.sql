CREATE TABLE notification_channels (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    kind TEXT NOT NULL CHECK (kind IN ('telegram', 'webhook')),
    destination TEXT NOT NULL CHECK (btrim(destination) <> ''),
    secret_ref TEXT CHECK (secret_ref IS NULL OR btrim(secret_ref) <> ''),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    archived_at TIMESTAMPTZ,
    UNIQUE (id, project_id),
    CHECK (
        (kind = 'telegram' AND destination ~ '^-?[0-9]+$')
        OR (kind = 'webhook' AND destination ~ '^https://[^/?#[:space:]]+')
    )
);

CREATE INDEX notification_channels_project_idx ON notification_channels (project_id);

CREATE TABLE notification_rules (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    scope TEXT NOT NULL DEFAULT 'project' CHECK (scope IN ('project', 'monitors')),
    severities TEXT[] NOT NULL DEFAULT ARRAY['warning', 'critical'],
    event_types TEXT[] NOT NULL DEFAULT ARRAY['incident_opened', 'severity_changed', 'incident_resolved'],
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (id, project_id),
    UNIQUE (id, project_id, scope),
    CHECK (
        cardinality(severities) > 0 AND array_ndims(severities) = 1
        AND array_position(severities, NULL) IS NULL
        AND severities <@ ARRAY['warning', 'critical']::TEXT[]
    ),
    CHECK (
        cardinality(event_types) > 0 AND array_ndims(event_types) = 1
        AND array_position(event_types, NULL) IS NULL
        AND event_types <@ ARRAY['incident_opened', 'severity_changed', 'incident_resolved']::TEXT[]
    )
);

CREATE INDEX notification_rules_project_idx ON notification_rules (project_id) WHERE enabled;

CREATE TABLE notification_rule_monitors (
    rule_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    monitor_id INTEGER NOT NULL,
    scope TEXT NOT NULL DEFAULT 'monitors' CHECK (scope = 'monitors'),
    PRIMARY KEY (rule_id, monitor_id),
    FOREIGN KEY (rule_id, project_id, scope)
        REFERENCES notification_rules(id, project_id, scope) ON DELETE CASCADE,
    FOREIGN KEY (monitor_id, project_id) REFERENCES monitors(id, project_id) ON DELETE RESTRICT
);

CREATE INDEX notification_rule_monitors_monitor_idx ON notification_rule_monitors (monitor_id);

CREATE TABLE notification_rule_channels (
    rule_id BIGINT NOT NULL,
    project_id BIGINT NOT NULL,
    channel_id BIGINT NOT NULL,
    PRIMARY KEY (rule_id, channel_id),
    FOREIGN KEY (rule_id, project_id) REFERENCES notification_rules(id, project_id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id, project_id) REFERENCES notification_channels(id, project_id) ON DELETE RESTRICT
);

CREATE INDEX notification_rule_channels_channel_idx ON notification_rule_channels (channel_id);

CREATE TABLE notification_deliveries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL,
    event_id UUID NOT NULL,
    channel_id BIGINT NOT NULL,
    channel_kind TEXT NOT NULL CHECK (channel_kind IN ('telegram', 'webhook')),
    destination TEXT NOT NULL CHECK (btrim(destination) <> ''),
    secret_ref TEXT,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'suppressed', 'cancelled')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_attempt_at TIMESTAMPTZ,
    last_error TEXT,
    lease_token UUID UNIQUE,
    lease_owner TEXT,
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMPTZ,
    FOREIGN KEY (event_id, project_id) REFERENCES monitor_events(id, project_id) ON DELETE RESTRICT,
    FOREIGN KEY (channel_id, project_id) REFERENCES notification_channels(id, project_id) ON DELETE RESTRICT,
    UNIQUE (event_id, channel_id),
    CHECK (
        (channel_kind = 'telegram' AND destination ~ '^-?[0-9]+$')
        OR (channel_kind = 'webhook' AND destination ~ '^https://[^/?#[:space:]]+')
    ),
    CHECK (
        (status = 'processing' AND lease_token IS NOT NULL AND lease_owner IS NOT NULL
            AND btrim(lease_owner) <> '' AND lease_expires_at IS NOT NULL)
        OR (status <> 'processing' AND lease_token IS NULL AND lease_owner IS NULL
            AND lease_expires_at IS NULL)
    ),
    CHECK ((status = 'sent') = (sent_at IS NOT NULL)),
    CHECK (sent_at IS NULL OR sent_at >= created_at),
    CHECK (last_attempt_at IS NULL OR last_attempt_at >= created_at),
    CHECK ((attempts = 0) = (last_attempt_at IS NULL)),
    CHECK (status NOT IN ('sent', 'failed') OR attempts > 0)
);

CREATE INDEX notification_deliveries_pending_idx ON notification_deliveries (next_attempt_at, id)
    WHERE status = 'pending';
CREATE INDEX notification_deliveries_lease_idx ON notification_deliveries (lease_expires_at)
    WHERE status = 'processing';
CREATE INDEX notification_deliveries_channel_idx ON notification_deliveries (channel_id, created_at DESC);
CREATE INDEX notification_deliveries_project_idx ON notification_deliveries (project_id, created_at DESC);

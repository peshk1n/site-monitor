CREATE TABLE monitor_runtime (
    monitor_id INTEGER PRIMARY KEY REFERENCES monitors(id) ON DELETE CASCADE,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    active_run_id UUID UNIQUE,
    lease_owner TEXT,
    lease_expires_at TIMESTAMPTZ,
    running_config_version BIGINT CHECK (running_config_version > 0),
    health_status TEXT NOT NULL DEFAULT 'unknown'
        CHECK (health_status IN ('unknown', 'up', 'degraded', 'down')),
    state_config_version BIGINT NOT NULL CHECK (state_config_version > 0),
    consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    consecutive_successes INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_successes >= 0),
    last_check_id BIGINT,
    last_observed_at TIMESTAMPTZ,
    fresh_until TIMESTAMPTZ,
    state_changed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (last_check_id, monitor_id) REFERENCES check_results(id, monitor_id)
        ON DELETE SET NULL (last_check_id),
    CHECK (consecutive_failures = 0 OR consecutive_successes = 0),
    CHECK (
        (active_run_id IS NULL AND lease_owner IS NULL AND lease_expires_at IS NULL
            AND running_config_version IS NULL)
        OR (active_run_id IS NOT NULL AND lease_owner IS NOT NULL AND btrim(lease_owner) <> ''
            AND lease_expires_at IS NOT NULL AND running_config_version IS NOT NULL)
    ),
    CHECK (
        (last_observed_at IS NULL AND fresh_until IS NULL)
        OR (last_observed_at IS NOT NULL AND fresh_until IS NOT NULL AND fresh_until >= last_observed_at)
    )
);

INSERT INTO monitor_runtime (monitor_id, state_config_version)
    SELECT id, config_version FROM monitors;

CREATE INDEX monitor_runtime_due_idx ON monitor_runtime (next_run_at, monitor_id)
    WHERE active_run_id IS NULL;
CREATE INDEX monitor_runtime_lease_idx ON monitor_runtime (lease_expires_at)
    WHERE active_run_id IS NOT NULL;

ALTER TABLE monitor_runtime
    DROP COLUMN consecutive_failures,
    DROP COLUMN consecutive_successes,
    ADD COLUMN evaluation_started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN candidate_status TEXT,
    ADD COLUMN candidate_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN candidate_since TIMESTAMPTZ,
    ADD CONSTRAINT monitor_runtime_candidate_status_check
        CHECK (candidate_status IN ('up', 'degraded', 'down')),
    ADD CONSTRAINT monitor_runtime_candidate_check CHECK (
        (candidate_status IS NULL AND candidate_count = 0 AND candidate_since IS NULL)
        OR (candidate_status IS NOT NULL AND candidate_status <> health_status
            AND candidate_count BETWEEN 1 AND 100 AND candidate_since IS NOT NULL
            AND last_observed_at IS NOT NULL AND candidate_since <= last_observed_at)
    );

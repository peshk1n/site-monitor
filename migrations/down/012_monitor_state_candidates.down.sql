ALTER TABLE monitor_runtime
    DROP COLUMN evaluation_started_at,
    DROP COLUMN candidate_status,
    DROP COLUMN candidate_count,
    DROP COLUMN candidate_since,
    ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    ADD COLUMN consecutive_successes INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_successes >= 0),
    ADD CONSTRAINT monitor_runtime_consecutive_check
        CHECK (consecutive_failures = 0 OR consecutive_successes = 0);

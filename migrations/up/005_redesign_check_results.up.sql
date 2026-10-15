ALTER TABLE checks RENAME TO check_results;
ALTER TABLE check_results RENAME COLUMN checked_at TO observed_at;
ALTER TABLE check_results RENAME COLUMN error TO error_message;
ALTER TABLE check_results
    DROP CONSTRAINT checks_monitor_id_fkey,
    ALTER COLUMN id TYPE BIGINT,
    ALTER COLUMN monitor_id SET NOT NULL,
    ALTER COLUMN observed_at TYPE TIMESTAMPTZ USING observed_at AT TIME ZONE 'UTC',
    ADD COLUMN run_id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN origin TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN config_version BIGINT,
    ADD COLUMN scheduled_at TIMESTAMPTZ,
    ADD COLUMN started_at TIMESTAMPTZ,
    ADD COLUMN finished_at TIMESTAMPTZ,
    ADD COLUMN recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN outcome TEXT,
    ADD COLUMN error_kind TEXT,
    ADD COLUMN accepted BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN discard_reason TEXT;

ALTER SEQUENCE checks_id_seq AS BIGINT;
ALTER SEQUENCE checks_id_seq RENAME TO check_results_id_seq;

UPDATE check_results SET outcome = CASE WHEN is_up THEN 'success' ELSE 'failure' END;
UPDATE check_results SET status_code = NULL WHERE status_code = 0;
UPDATE check_results SET error_message = NULL WHERE error_message = '';

ALTER TABLE check_results
    DROP COLUMN is_up,
    ALTER COLUMN observed_at DROP DEFAULT,
    ALTER COLUMN origin SET DEFAULT 'scheduled',
    ALTER COLUMN outcome SET NOT NULL,
    ADD CONSTRAINT check_results_monitor_fk FOREIGN KEY (monitor_id)
        REFERENCES monitors(id) ON DELETE RESTRICT,
    ADD CONSTRAINT check_results_monitor_key UNIQUE (id, monitor_id),
    ADD CONSTRAINT check_results_run_key UNIQUE (run_id),
    ADD CONSTRAINT check_results_origin_check CHECK (origin IN ('legacy', 'scheduled', 'manual')),
    ADD CONSTRAINT check_results_version_check CHECK (config_version > 0),
    ADD CONSTRAINT check_results_outcome_check
        CHECK (outcome IN ('success', 'warning', 'failure', 'error', 'cancelled')),
    ADD CONSTRAINT check_results_error_kind_check CHECK (
        error_kind IN ('timeout', 'dns', 'tls', 'connection', 'http_status',
                       'body_mismatch', 'execution', 'cancelled')
    ),
    ADD CONSTRAINT check_results_code_check CHECK (status_code BETWEEN 100 AND 599),
    ADD CONSTRAINT check_results_response_check CHECK (response_ms >= 0),
    ADD CONSTRAINT check_results_observation_check CHECK (
        origin = 'legacy' OR
        (config_version IS NOT NULL AND started_at IS NOT NULL AND finished_at IS NOT NULL
         AND observed_at IS NOT NULL AND response_ms IS NOT NULL)
    ),
    ADD CONSTRAINT check_results_schedule_check
        CHECK (origin <> 'scheduled' OR scheduled_at IS NOT NULL),
    ADD CONSTRAINT check_results_times_check CHECK (
        finished_at >= started_at AND observed_at >= started_at AND observed_at <= finished_at
    ),
    ADD CONSTRAINT check_results_discard_check CHECK (
        (accepted AND discard_reason IS NULL)
        OR (NOT accepted AND discard_reason IS NOT NULL AND btrim(discard_reason) <> '')
    );

CREATE INDEX check_results_history_idx ON check_results (monitor_id, observed_at DESC, id DESC);
CREATE INDEX check_results_retention_idx ON check_results (recorded_at);

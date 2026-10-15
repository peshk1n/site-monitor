DROP INDEX check_results_history_idx;
DROP INDEX check_results_retention_idx;
ALTER TABLE check_results
    DROP CONSTRAINT check_results_code_check,
    ADD COLUMN is_up BOOLEAN;
UPDATE check_results SET
    is_up = outcome IN ('success', 'warning'),
    status_code = COALESCE(status_code, 0),
    response_ms = COALESCE(response_ms, 0),
    error_message = COALESCE(error_message, '');
ALTER TABLE check_results
    ALTER COLUMN is_up SET NOT NULL,
    DROP CONSTRAINT check_results_monitor_fk,
    DROP CONSTRAINT check_results_monitor_key,
    DROP CONSTRAINT check_results_response_check,
    DROP COLUMN run_id,
    DROP COLUMN origin,
    DROP COLUMN config_version,
    DROP COLUMN scheduled_at,
    DROP COLUMN started_at,
    DROP COLUMN finished_at,
    DROP COLUMN recorded_at,
    DROP COLUMN outcome,
    DROP COLUMN error_kind,
    DROP COLUMN accepted,
    DROP COLUMN discard_reason,
    ALTER COLUMN id TYPE INTEGER,
    ALTER COLUMN monitor_id DROP NOT NULL,
    ALTER COLUMN observed_at TYPE TIMESTAMP USING observed_at AT TIME ZONE 'UTC',
    ALTER COLUMN observed_at SET DEFAULT NOW(),
    ADD CONSTRAINT checks_monitor_id_fkey FOREIGN KEY (monitor_id)
        REFERENCES monitors(id) ON DELETE CASCADE;
ALTER SEQUENCE check_results_id_seq AS INTEGER;
ALTER SEQUENCE check_results_id_seq RENAME TO checks_id_seq;
ALTER TABLE check_results RENAME COLUMN observed_at TO checked_at;
ALTER TABLE check_results RENAME COLUMN error_message TO error;
ALTER TABLE check_results RENAME TO checks;

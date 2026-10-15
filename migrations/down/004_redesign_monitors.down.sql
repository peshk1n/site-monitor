DROP TRIGGER monitors_config_version ON monitors;
DROP FUNCTION update_monitor_config_version();
DROP INDEX monitors_project_idx;
ALTER TABLE monitors
    DROP CONSTRAINT monitors_project_key,
    DROP CONSTRAINT monitors_url_check,
    DROP CONSTRAINT monitors_interval_check,
    DROP CONSTRAINT monitors_timeout_check,
    DROP COLUMN project_id,
    DROP COLUMN name,
    DROP COLUMN expected_status_codes,
    DROP COLUMN body_match_type,
    DROP COLUMN body_pattern,
    DROP COLUMN warning_response_ms,
    DROP COLUMN failure_threshold,
    DROP COLUMN recovery_threshold,
    DROP COLUMN max_body_bytes,
    DROP COLUMN max_redirects,
    DROP COLUMN config_version,
    DROP COLUMN updated_at,
    DROP COLUMN archived_at,
    ALTER COLUMN created_at DROP NOT NULL,
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';
ALTER TABLE monitors RENAME COLUMN interval_seconds TO interval;
ALTER TABLE monitors RENAME COLUMN timeout_seconds TO timeout;

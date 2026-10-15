ALTER TABLE monitors RENAME COLUMN interval TO interval_seconds;
ALTER TABLE monitors RENAME COLUMN timeout TO timeout_seconds;
ALTER TABLE monitors
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ADD COLUMN project_id BIGINT REFERENCES projects(id) ON DELETE RESTRICT,
    ADD COLUMN name TEXT,
    ADD COLUMN expected_status_codes SMALLINT[] NOT NULL DEFAULT ARRAY[200]::SMALLINT[],
    ADD COLUMN body_match_type TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN body_pattern TEXT,
    ADD COLUMN warning_response_ms INTEGER,
    ADD COLUMN failure_threshold INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN recovery_threshold INTEGER NOT NULL DEFAULT 2,
    ADD COLUMN max_body_bytes INTEGER NOT NULL DEFAULT 1048576,
    ADD COLUMN max_redirects SMALLINT NOT NULL DEFAULT 5,
    ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN archived_at TIMESTAMPTZ;

UPDATE monitors SET project_id = (SELECT id FROM projects WHERE slug = 'default'), name = url;
UPDATE monitors SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;

ALTER TABLE monitors
    ALTER COLUMN project_id SET NOT NULL,
    ALTER COLUMN name SET NOT NULL,
    ALTER COLUMN created_at SET NOT NULL,
    ADD CONSTRAINT monitors_project_key UNIQUE (id, project_id),
    ADD CONSTRAINT monitors_name_check CHECK (btrim(name) <> ''),
    ADD CONSTRAINT monitors_url_check CHECK (url ~ '^https?://[^/?#[:space:]]+'),
    ADD CONSTRAINT monitors_interval_check CHECK (interval_seconds BETWEEN 1 AND 86400),
    ADD CONSTRAINT monitors_timeout_check CHECK (timeout_seconds BETWEEN 1 AND 300),
    ADD CONSTRAINT monitors_codes_check CHECK (
        cardinality(expected_status_codes) > 0
        AND array_ndims(expected_status_codes) = 1
        AND array_position(expected_status_codes, NULL) IS NULL
        AND 100 <= ALL(expected_status_codes) AND 599 >= ALL(expected_status_codes)
    ),
    ADD CONSTRAINT monitors_body_check CHECK (
        (body_match_type = 'none' AND body_pattern IS NULL)
        OR (body_match_type IN ('substring', 'regexp')
            AND body_pattern IS NOT NULL AND body_pattern <> '')
    ),
    ADD CONSTRAINT monitors_warning_check CHECK (
        warning_response_ms IS NULL
        OR (warning_response_ms > 0 AND warning_response_ms <= timeout_seconds * 1000)
    ),
    ADD CONSTRAINT monitors_thresholds_check CHECK (
        failure_threshold BETWEEN 1 AND 100 AND recovery_threshold BETWEEN 1 AND 100
    ),
    ADD CONSTRAINT monitors_body_size_check CHECK (max_body_bytes BETWEEN 1 AND 10485760),
    ADD CONSTRAINT monitors_redirects_check CHECK (max_redirects BETWEEN 0 AND 10),
    ADD CONSTRAINT monitors_version_check CHECK (config_version > 0);

CREATE INDEX monitors_project_idx ON monitors (project_id, id);

CREATE FUNCTION update_monitor_config_version() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.project_id, NEW.url, NEW.interval_seconds, NEW.timeout_seconds,
           NEW.expected_status_codes, NEW.body_match_type, NEW.body_pattern,
           NEW.warning_response_ms, NEW.failure_threshold, NEW.recovery_threshold,
           NEW.max_body_bytes, NEW.max_redirects, NEW.is_active, NEW.archived_at)
       IS DISTINCT FROM
       ROW(OLD.project_id, OLD.url, OLD.interval_seconds, OLD.timeout_seconds,
           OLD.expected_status_codes, OLD.body_match_type, OLD.body_pattern,
           OLD.warning_response_ms, OLD.failure_threshold, OLD.recovery_threshold,
           OLD.max_body_bytes, OLD.max_redirects, OLD.is_active, OLD.archived_at) THEN
        NEW.config_version := OLD.config_version + 1;
    ELSE
        NEW.config_version := OLD.config_version;
    END IF;
    NEW.updated_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER monitors_config_version
    BEFORE UPDATE ON monitors FOR EACH ROW EXECUTE FUNCTION update_monitor_config_version();

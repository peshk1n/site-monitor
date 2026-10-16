CREATE TABLE creation_requests (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    request_key UUID NOT NULL,
    operation TEXT NOT NULL CHECK (operation IN ('create_project', 'create_monitor')),
    request_hash TEXT NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    response JSONB CHECK (jsonb_typeof(response) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, request_key)
);

CREATE INDEX check_results_latest_accepted_idx
    ON check_results (monitor_id, recorded_at DESC, id DESC) WHERE accepted;

CREATE TABLE checks (
    id          SERIAL PRIMARY KEY,
    monitor_id  INTEGER REFERENCES monitors(id) ON DELETE CASCADE,
    status_code INTEGER,
    response_ms INTEGER,  
    is_up       BOOLEAN NOT NULL,
    error       TEXT,      
    checked_at  TIMESTAMP DEFAULT NOW()
);
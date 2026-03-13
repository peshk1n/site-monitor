CREATE TABLE monitors (
    id          SERIAL PRIMARY KEY,
    url         TEXT NOT NULL,
    interval    INTEGER NOT NULL DEFAULT 60, 
    timeout     INTEGER NOT NULL DEFAULT 10,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMP DEFAULT NOW()
);
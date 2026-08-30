CREATE TABLE IF NOT EXISTS checkpoints (
    name TEXT PRIMARY KEY,
    block_number BIGINT NOT NULL,
    block_hash TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
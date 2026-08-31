CREATE TABLE IF NOT EXISTS indexed_blocks (
    block_number BIGINT PRIMARY KEY,
    block_hash TEXT NOT NULL,
    parent_hash TEXT NOT NULL,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK (block_number >= 0)
);

CREATE INDEX IF NOT EXISTS idx_indexed_blocks_hash
    ON indexed_blocks (block_hash);

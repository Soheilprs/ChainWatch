CREATE TABLE IF NOT EXISTS token_metadata (
    token_address TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    decimals SMALLINT NOT NULL
        CHECK (decimals >= 0 AND decimals <= 255),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
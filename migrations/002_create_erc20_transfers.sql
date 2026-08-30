CREATE TABLE IF NOT EXISTS erc20_transfers (
    transaction_hash TEXT NOT NULL,
    log_index INTEGER NOT NULL,

    block_number BIGINT NOT NULL,
    block_hash TEXT NOT NULL,

    token_address TEXT NOT NULL,
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,

    value NUMERIC(78, 0) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (transaction_hash, log_index)
);

CREATE INDEX IF NOT EXISTS idx_erc20_transfers_block_number
    ON erc20_transfers (block_number);

CREATE INDEX IF NOT EXISTS idx_erc20_transfers_token_address
    ON erc20_transfers (token_address);

CREATE INDEX IF NOT EXISTS idx_erc20_transfers_from_address
    ON erc20_transfers (from_address);

CREATE INDEX IF NOT EXISTS idx_erc20_transfers_to_address
    ON erc20_transfers (to_address);
CREATE INDEX IF NOT EXISTS idx_erc20_transfers_token_address_lower
    ON erc20_transfers (LOWER(token_address));

CREATE INDEX IF NOT EXISTS idx_erc20_transfers_from_address_lower
    ON erc20_transfers (LOWER(from_address));

CREATE INDEX IF NOT EXISTS idx_erc20_transfers_to_address_lower
    ON erc20_transfers (LOWER(to_address));

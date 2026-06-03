CREATE TABLE IF NOT EXISTS inventories (
    sku_id      BIGINT PRIMARY KEY,
    total       BIGINT NOT NULL CHECK (total >= 0),
    available   BIGINT NOT NULL CHECK (available >= 0),
    locked      BIGINT NOT NULL CHECK (locked >= 0),
    sold        BIGINT NOT NULL CHECK (sold >= 0),
    version     BIGINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (total = available + locked + sold)
);

CREATE TABLE IF NOT EXISTS inventory_deductions (
    id              BIGSERIAL PRIMARY KEY,
    request_id      TEXT NOT NULL UNIQUE,
    sku_id          BIGINT NOT NULL REFERENCES inventories(sku_id),
    quantity        BIGINT NOT NULL CHECK (quantity > 0),
    status          TEXT NOT NULL CHECK (status IN ('LOCKED', 'RELEASED', 'CONFIRMED')),
    release_request_id TEXT UNIQUE,
    confirm_request_id TEXT UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inventory_deductions_sku_status
    ON inventory_deductions (sku_id, status);

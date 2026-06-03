CREATE TABLE IF NOT EXISTS inventories (
    sku_id      BIGINT PRIMARY KEY,
    total       BIGINT NOT NULL CHECK (total >= 0),
    -- sq: salable quantity. Available database stock that has not been
    -- deducted by successful orders. The Redis pre-lock quantity (lq) is a
    -- subset of sq, so database fallback deductions must use sq - lq.
    sq          BIGINT NOT NULL CHECK (sq >= 0),
    -- wq: withholding quantity. Stock deducted by created orders and waiting
    -- for final confirmation or release.
    wq          BIGINT NOT NULL CHECK (wq >= 0),
    -- oq: occupied/output quantity. Confirmed sold stock.
    oq          BIGINT NOT NULL CHECK (oq >= 0),
    -- lq: locked quantity. Stock pre-locked into Redis buckets for hot SKU
    -- high-concurrency deductions.
    lq          BIGINT NOT NULL DEFAULT 0 CHECK (lq >= 0),
    version     BIGINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (total = sq + wq + oq),
    CHECK (lq <= sq)
);

CREATE TABLE IF NOT EXISTS inventory_deductions (
    id              BIGSERIAL PRIMARY KEY,
    request_id      TEXT NOT NULL UNIQUE,
    sku_id          BIGINT NOT NULL REFERENCES inventories(sku_id),
    quantity        BIGINT NOT NULL CHECK (quantity > 0),
    status          TEXT NOT NULL CHECK (status IN ('LOCKED', 'RELEASED', 'CONFIRMED')),
    bucket_key      TEXT,
    release_request_id TEXT UNIQUE,
    confirm_request_id TEXT UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inventory_deductions_sku_status
    ON inventory_deductions (sku_id, status);

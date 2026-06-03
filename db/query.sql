-- name: CreateInventory :one
INSERT INTO inventories (sku_id, total, sq, wq, oq, lq)
VALUES ($1, $2, $2, 0, 0, 0)
ON CONFLICT (sku_id) DO UPDATE
SET total = EXCLUDED.total,
    sq = EXCLUDED.total - inventories.wq - inventories.oq,
    lq = LEAST(inventories.lq, EXCLUDED.total - inventories.wq - inventories.oq),
    version = inventories.version + 1,
    updated_at = now()
WHERE EXCLUDED.total >= inventories.wq + inventories.oq
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

-- name: GetInventory :one
SELECT sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at
FROM inventories
WHERE sku_id = $1;

-- name: DeductInventory :one
UPDATE inventories
SET sq = sq - sqlc.arg(quantity),
    wq = wq + sqlc.arg(quantity),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND sq - lq >= sqlc.arg(quantity)
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

-- name: DeductLockedInventory :one
UPDATE inventories
SET sq = sq - sqlc.arg(quantity),
    lq = lq - sqlc.arg(quantity),
    wq = wq + sqlc.arg(quantity),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND lq >= sqlc.arg(quantity)
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

-- name: LockRedisBucketStock :one
UPDATE inventories
SET lq = lq + sqlc.arg(quantity),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND sq - lq >= sqlc.arg(quantity)
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at, sqlc.arg(quantity)::bigint AS locked_quantity;

-- name: InsertDeduction :one
INSERT INTO inventory_deductions (request_id, sku_id, quantity, status, bucket_key)
VALUES ($1, $2, $3, 'LOCKED', $4)
RETURNING id, request_id, sku_id, quantity, status, bucket_key, release_request_id, confirm_request_id, created_at, updated_at;

-- name: GetDeductionByRequestID :one
SELECT id, request_id, sku_id, quantity, status, bucket_key, release_request_id, confirm_request_id, created_at, updated_at
FROM inventory_deductions
WHERE request_id = $1;

-- name: LockDeductionByRequestID :one
SELECT id, request_id, sku_id, quantity, status, bucket_key, release_request_id, confirm_request_id, created_at, updated_at
FROM inventory_deductions
WHERE request_id = $1
FOR UPDATE;

-- name: ReleaseLockedInventory :one
UPDATE inventories
SET lq = lq - sqlc.arg(quantity),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND lq >= sqlc.arg(quantity)
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

-- name: ReleaseInventory :one
UPDATE inventories
SET sq = sq + sqlc.arg(quantity),
    wq = wq - sqlc.arg(quantity),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND wq >= sqlc.arg(quantity)
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

-- name: MarkDeductionReleased :one
UPDATE inventory_deductions
SET status = 'RELEASED', release_request_id = $2, updated_at = now()
WHERE request_id = $1 AND status = 'LOCKED'
RETURNING id, request_id, sku_id, quantity, status, bucket_key, release_request_id, confirm_request_id, created_at, updated_at;

-- name: ConfirmLockedInventory :one
UPDATE inventories
SET sq = sq - sqlc.arg(quantity),
    lq = lq - sqlc.arg(quantity),
    oq = oq + sqlc.arg(quantity),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND lq >= sqlc.arg(quantity)
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

-- name: ConfirmInventory :one
UPDATE inventories
SET wq = wq - sqlc.arg(quantity),
    oq = oq + sqlc.arg(quantity),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND wq >= sqlc.arg(quantity)
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

-- name: MarkDeductionConfirmed :one
UPDATE inventory_deductions
SET status = 'CONFIRMED', confirm_request_id = $2, updated_at = now()
WHERE request_id = $1 AND status = 'LOCKED'
RETURNING id, request_id, sku_id, quantity, status, bucket_key, release_request_id, confirm_request_id, created_at, updated_at;

-- name: EditInventory :one
UPDATE inventories
SET total = $2,
    sq = $2 - wq - oq,
    lq = LEAST(lq, $2 - wq - oq),
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1
  AND version = $3
  AND $2 >= wq + oq
RETURNING sku_id, total, sq AS available, wq AS locked, oq AS sold, lq AS pre_locked, version, created_at, updated_at;

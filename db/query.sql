-- name: CreateInventory :one
INSERT INTO inventories (sku_id, total, available, locked, sold)
VALUES ($1, $2, $2, 0, 0)
ON CONFLICT (sku_id) DO UPDATE
SET total = EXCLUDED.total,
    available = EXCLUDED.total - inventories.locked - inventories.sold,
    version = inventories.version + 1,
    updated_at = now()
WHERE EXCLUDED.total >= inventories.locked + inventories.sold
RETURNING sku_id, total, available, locked, sold, version, created_at, updated_at;

-- name: GetInventory :one
SELECT sku_id, total, available, locked, sold, version, created_at, updated_at
FROM inventories
WHERE sku_id = $1;

-- name: DeductInventory :one
UPDATE inventories
SET available = available - $2,
    locked = locked + $2,
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND available >= $2
RETURNING sku_id, total, available, locked, sold, version, created_at, updated_at;

-- name: InsertDeduction :one
INSERT INTO inventory_deductions (request_id, sku_id, quantity, status)
VALUES ($1, $2, $3, 'LOCKED')
RETURNING id, request_id, sku_id, quantity, status, release_request_id, confirm_request_id, created_at, updated_at;

-- name: GetDeductionByRequestID :one
SELECT id, request_id, sku_id, quantity, status, release_request_id, confirm_request_id, created_at, updated_at
FROM inventory_deductions
WHERE request_id = $1;

-- name: LockDeductionByRequestID :one
SELECT id, request_id, sku_id, quantity, status, release_request_id, confirm_request_id, created_at, updated_at
FROM inventory_deductions
WHERE request_id = $1
FOR UPDATE;

-- name: ReleaseInventory :one
UPDATE inventories
SET available = available + $2,
    locked = locked - $2,
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND locked >= $2
RETURNING sku_id, total, available, locked, sold, version, created_at, updated_at;

-- name: MarkDeductionReleased :one
UPDATE inventory_deductions
SET status = 'RELEASED', release_request_id = $2, updated_at = now()
WHERE request_id = $1 AND status = 'LOCKED'
RETURNING id, request_id, sku_id, quantity, status, release_request_id, confirm_request_id, created_at, updated_at;

-- name: ConfirmInventory :one
UPDATE inventories
SET locked = locked - $2,
    sold = sold + $2,
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1 AND locked >= $2
RETURNING sku_id, total, available, locked, sold, version, created_at, updated_at;

-- name: MarkDeductionConfirmed :one
UPDATE inventory_deductions
SET status = 'CONFIRMED', confirm_request_id = $2, updated_at = now()
WHERE request_id = $1 AND status = 'LOCKED'
RETURNING id, request_id, sku_id, quantity, status, release_request_id, confirm_request_id, created_at, updated_at;

-- name: EditInventory :one
UPDATE inventories
SET total = $2,
    available = $2 - locked - sold,
    version = version + 1,
    updated_at = now()
WHERE sku_id = $1
  AND version = $3
  AND $2 >= locked + sold
RETURNING sku_id, total, available, locked, sold, version, created_at, updated_at;

package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"inventory-service/internal/biz"
	"inventory-service/internal/data/sqlc"
)

type InventoryRepo struct {
	db *sql.DB
	q  *sqlc.Queries
}

func NewInventoryRepo(db *sql.DB) *InventoryRepo { return &InventoryRepo{db: db, q: sqlc.New(db)} }

func (r *InventoryRepo) CreateOrReplace(ctx context.Context, skuID, total int64) (biz.Inventory, error) {
	inv, err := r.q.CreateInventory(ctx, sqlc.CreateInventoryParams{SkuID: skuID, Total: total})
	if errors.Is(err, sql.ErrNoRows) {
		return biz.Inventory{}, biz.ErrInvalidState
	}
	return toBizInventory(inv), wrapDBErr(err)
}

func (r *InventoryRepo) Get(ctx context.Context, skuID int64) (biz.Inventory, error) {
	inv, err := r.q.GetInventory(ctx, skuID)
	if errors.Is(err, sql.ErrNoRows) {
		return biz.Inventory{}, biz.ErrNotFound
	}
	return toBizInventory(inv), wrapDBErr(err)
}

func (r *InventoryRepo) Deduct(ctx context.Context, requestID string, skuID, quantity int64) (biz.Inventory, biz.Deduction, error) {
	var outInv biz.Inventory
	var outDed biz.Deduction
	err := r.withTx(ctx, func(q *sqlc.Queries) error {
		existing, err := q.GetDeductionByRequestID(ctx, requestID)
		if err == nil {
			inv, invErr := q.GetInventory(ctx, existing.SkuID)
			if invErr != nil {
				return invErr
			}
			outInv, outDed = toBizInventory(inv), toBizDeduction(existing)
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		inv, err := q.DeductInventory(ctx, sqlc.DeductInventoryParams{SkuID: skuID, Quantity: quantity})
		if errors.Is(err, sql.ErrNoRows) {
			return biz.ErrInsufficientStock
		}
		if err != nil {
			return err
		}
		ded, err := q.InsertDeduction(ctx, sqlc.InsertDeductionParams{RequestID: requestID, SkuID: skuID, Quantity: quantity})
		if err != nil {
			return err
		}
		outInv, outDed = toBizInventory(inv), toBizDeduction(ded)
		return nil
	})
	if isDuplicateKey(err) {
		existing, getErr := r.q.GetDeductionByRequestID(ctx, requestID)
		if getErr != nil {
			return outInv, outDed, wrapDBErr(err)
		}
		inv, getErr := r.q.GetInventory(ctx, existing.SkuID)
		if getErr != nil {
			return outInv, outDed, wrapDBErr(getErr)
		}
		return toBizInventory(inv), toBizDeduction(existing), nil
	}
	return outInv, outDed, wrapDBErr(err)
}

func (r *InventoryRepo) Release(ctx context.Context, releaseRequestID, deductionRequestID string) (biz.Inventory, biz.Deduction, error) {
	var outInv biz.Inventory
	var outDed biz.Deduction
	err := r.withTx(ctx, func(q *sqlc.Queries) error {
		ded, err := q.LockDeductionByRequestID(ctx, deductionRequestID)
		if errors.Is(err, sql.ErrNoRows) {
			return biz.ErrNotFound
		}
		if err != nil {
			return err
		}
		if ded.Status == "RELEASED" || (ded.ReleaseRequestID.Valid && ded.ReleaseRequestID.String == releaseRequestID) {
			inv, invErr := q.GetInventory(ctx, ded.SkuID)
			if invErr != nil {
				return invErr
			}
			outInv, outDed = toBizInventory(inv), toBizDeduction(ded)
			return nil
		}
		if ded.Status != "LOCKED" {
			return biz.ErrInvalidState
		}
		inv, err := q.ReleaseInventory(ctx, sqlc.ReleaseInventoryParams{SkuID: ded.SkuID, Quantity: ded.Quantity})
		if err != nil {
			return err
		}
		ded, err = q.MarkDeductionReleased(ctx, sqlc.MarkDeductionReleasedParams{RequestID: deductionRequestID, ReleaseRequestID: sql.NullString{String: releaseRequestID, Valid: true}})
		if err != nil {
			return err
		}
		outInv, outDed = toBizInventory(inv), toBizDeduction(ded)
		return nil
	})
	return outInv, outDed, wrapDBErr(err)
}

func (r *InventoryRepo) Confirm(ctx context.Context, confirmRequestID, deductionRequestID string) (biz.Inventory, biz.Deduction, error) {
	var outInv biz.Inventory
	var outDed biz.Deduction
	err := r.withTx(ctx, func(q *sqlc.Queries) error {
		ded, err := q.LockDeductionByRequestID(ctx, deductionRequestID)
		if errors.Is(err, sql.ErrNoRows) {
			return biz.ErrNotFound
		}
		if err != nil {
			return err
		}
		if ded.Status == "CONFIRMED" || (ded.ConfirmRequestID.Valid && ded.ConfirmRequestID.String == confirmRequestID) {
			inv, invErr := q.GetInventory(ctx, ded.SkuID)
			if invErr != nil {
				return invErr
			}
			outInv, outDed = toBizInventory(inv), toBizDeduction(ded)
			return nil
		}
		if ded.Status != "LOCKED" {
			return biz.ErrInvalidState
		}
		inv, err := q.ConfirmInventory(ctx, sqlc.ConfirmInventoryParams{SkuID: ded.SkuID, Quantity: ded.Quantity})
		if err != nil {
			return err
		}
		ded, err = q.MarkDeductionConfirmed(ctx, sqlc.MarkDeductionConfirmedParams{RequestID: deductionRequestID, ConfirmRequestID: sql.NullString{String: confirmRequestID, Valid: true}})
		if err != nil {
			return err
		}
		outInv, outDed = toBizInventory(inv), toBizDeduction(ded)
		return nil
	})
	return outInv, outDed, wrapDBErr(err)
}

func (r *InventoryRepo) Edit(ctx context.Context, skuID, total, expectedVersion int64) (biz.Inventory, error) {
	inv, err := r.q.EditInventory(ctx, sqlc.EditInventoryParams{SkuID: skuID, Total: total, Version: expectedVersion})
	if errors.Is(err, sql.ErrNoRows) {
		return biz.Inventory{}, biz.ErrVersionConflict
	}
	return toBizInventory(inv), wrapDBErr(err)
}

func (r *InventoryRepo) withTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(r.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

func toBizInventory(i sqlc.Inventory) biz.Inventory {
	return biz.Inventory{SkuID: i.SkuID, Total: i.Total, Available: i.Available, Locked: i.Locked, Sold: i.Sold, Version: i.Version}
}
func toBizDeduction(d sqlc.InventoryDeduction) biz.Deduction {
	return biz.Deduction{RequestID: d.RequestID, SkuID: d.SkuID, Quantity: d.Quantity, Status: d.Status}
}

func wrapDBErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, biz.ErrInsufficientStock) || errors.Is(err, biz.ErrNotFound) || errors.Is(err, biz.ErrVersionConflict) || errors.Is(err, biz.ErrInvalidState) {
		return err
	}
	if isDuplicateKey(err) {
		return biz.ErrInvalidState
	}
	return fmt.Errorf("database operation failed: %w", err)
}

func isDuplicateKey(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}

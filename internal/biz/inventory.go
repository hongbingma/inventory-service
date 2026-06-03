package biz

import (
	"context"
	"errors"
)

var (
	ErrInvalidArgument   = errors.New("invalid argument")
	ErrNotFound          = errors.New("inventory not found")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrVersionConflict   = errors.New("inventory version conflict")
	ErrInvalidState      = errors.New("invalid deduction state")
)

type Inventory struct {
	SkuID     int64 `json:"sku_id"`
	Total     int64 `json:"total"`
	Available int64 `json:"available"`
	Locked    int64 `json:"locked"`
	Sold      int64 `json:"sold"`
	Version   int64 `json:"version"`
}

type Deduction struct {
	RequestID string `json:"request_id"`
	SkuID     int64  `json:"sku_id"`
	Quantity  int64  `json:"quantity"`
	Status    string `json:"status"`
}

type InventoryRepo interface {
	CreateOrReplace(ctx context.Context, skuID, total int64) (Inventory, error)
	Get(ctx context.Context, skuID int64) (Inventory, error)
	Deduct(ctx context.Context, requestID string, skuID, quantity int64) (Inventory, Deduction, error)
	Release(ctx context.Context, releaseRequestID, deductionRequestID string) (Inventory, Deduction, error)
	Confirm(ctx context.Context, confirmRequestID, deductionRequestID string) (Inventory, Deduction, error)
	Edit(ctx context.Context, skuID, total, expectedVersion int64) (Inventory, error)
}

type InventoryUsecase struct{ repo InventoryRepo }

func NewInventoryUsecase(repo InventoryRepo) *InventoryUsecase { return &InventoryUsecase{repo: repo} }

func (uc *InventoryUsecase) CreateOrReplace(ctx context.Context, skuID, total int64) (Inventory, error) {
	if skuID <= 0 || total < 0 {
		return Inventory{}, ErrInvalidArgument
	}
	return uc.repo.CreateOrReplace(ctx, skuID, total)
}

func (uc *InventoryUsecase) Get(ctx context.Context, skuID int64) (Inventory, error) {
	if skuID <= 0 {
		return Inventory{}, ErrInvalidArgument
	}
	return uc.repo.Get(ctx, skuID)
}

func (uc *InventoryUsecase) Deduct(ctx context.Context, requestID string, skuID, quantity int64) (Inventory, Deduction, error) {
	if requestID == "" || skuID <= 0 || quantity <= 0 {
		return Inventory{}, Deduction{}, ErrInvalidArgument
	}
	return uc.repo.Deduct(ctx, requestID, skuID, quantity)
}

func (uc *InventoryUsecase) Release(ctx context.Context, releaseRequestID, deductionRequestID string) (Inventory, Deduction, error) {
	if releaseRequestID == "" || deductionRequestID == "" {
		return Inventory{}, Deduction{}, ErrInvalidArgument
	}
	return uc.repo.Release(ctx, releaseRequestID, deductionRequestID)
}

func (uc *InventoryUsecase) Confirm(ctx context.Context, confirmRequestID, deductionRequestID string) (Inventory, Deduction, error) {
	if confirmRequestID == "" || deductionRequestID == "" {
		return Inventory{}, Deduction{}, ErrInvalidArgument
	}
	return uc.repo.Confirm(ctx, confirmRequestID, deductionRequestID)
}

func (uc *InventoryUsecase) Edit(ctx context.Context, skuID, total, expectedVersion int64) (Inventory, error) {
	if skuID <= 0 || total < 0 || expectedVersion <= 0 {
		return Inventory{}, ErrInvalidArgument
	}
	return uc.repo.Edit(ctx, skuID, total, expectedVersion)
}

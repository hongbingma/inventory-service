package service

import (
	"context"
	"errors"

	inventoryv1 "inventory-service/api/inventory/v1"
	"inventory-service/internal/biz"

	kerrors "github.com/go-kratos/kratos/v2/errors"
)

type InventoryService struct {
	inventoryv1.UnimplementedInventoryServer
	uc *biz.InventoryUsecase
}

func NewInventoryService(uc *biz.InventoryUsecase) *InventoryService {
	return &InventoryService{uc: uc}
}

func (s *InventoryService) CreateOrReplaceInventory(ctx context.Context, req *inventoryv1.CreateOrReplaceInventoryRequest) (*inventoryv1.InventoryReply, error) {
	inv, err := s.uc.CreateOrReplace(ctx, req.GetSkuId(), req.GetTotal())
	if err != nil {
		return nil, toKratosError(err)
	}
	return &inventoryv1.InventoryReply{Inventory: toInventoryDTO(inv)}, nil
}

func (s *InventoryService) DeductInventory(ctx context.Context, req *inventoryv1.DeductInventoryRequest) (*inventoryv1.InventoryOperationReply, error) {
	inv, ded, err := s.uc.Deduct(ctx, req.GetRequestId(), req.GetSkuId(), req.GetQuantity())
	if err != nil {
		return nil, toKratosError(err)
	}
	return &inventoryv1.InventoryOperationReply{Inventory: toInventoryDTO(inv), Deduction: toDeductionDTO(ded)}, nil
}

func (s *InventoryService) IncreaseInventory(ctx context.Context, req *inventoryv1.IncreaseInventoryRequest) (*inventoryv1.InventoryOperationReply, error) {
	inv, ded, err := s.uc.Release(ctx, req.GetRequestId(), req.GetDeductionRequestId())
	if err != nil {
		return nil, toKratosError(err)
	}
	return &inventoryv1.InventoryOperationReply{Inventory: toInventoryDTO(inv), Deduction: toDeductionDTO(ded)}, nil
}

func (s *InventoryService) ConfirmInventory(ctx context.Context, req *inventoryv1.ConfirmInventoryRequest) (*inventoryv1.InventoryOperationReply, error) {
	inv, ded, err := s.uc.Confirm(ctx, req.GetRequestId(), req.GetDeductionRequestId())
	if err != nil {
		return nil, toKratosError(err)
	}
	return &inventoryv1.InventoryOperationReply{Inventory: toInventoryDTO(inv), Deduction: toDeductionDTO(ded)}, nil
}

func (s *InventoryService) EditInventory(ctx context.Context, req *inventoryv1.EditInventoryRequest) (*inventoryv1.InventoryReply, error) {
	inv, err := s.uc.Edit(ctx, req.GetSkuId(), req.GetTotal(), req.GetExpectedVersion())
	if err != nil {
		return nil, toKratosError(err)
	}
	return &inventoryv1.InventoryReply{Inventory: toInventoryDTO(inv)}, nil
}

func (s *InventoryService) GetInventory(ctx context.Context, req *inventoryv1.GetInventoryRequest) (*inventoryv1.InventoryReply, error) {
	inv, err := s.uc.Get(ctx, req.GetSkuId())
	if err != nil {
		return nil, toKratosError(err)
	}
	return &inventoryv1.InventoryReply{Inventory: toInventoryDTO(inv)}, nil
}

func toInventoryDTO(inv biz.Inventory) *inventoryv1.InventoryDTO {
	return &inventoryv1.InventoryDTO{
		SkuId:     inv.SkuID,
		Total:     inv.Total,
		Available: inv.Available,
		Locked:    inv.Locked,
		Sold:      inv.Sold,
		Version:   inv.Version,
	}
}

func toDeductionDTO(ded biz.Deduction) *inventoryv1.DeductionDTO {
	return &inventoryv1.DeductionDTO{
		RequestId: ded.RequestID,
		SkuId:     ded.SkuID,
		Quantity:  ded.Quantity,
		Status:    ded.Status,
	}
}

func toKratosError(err error) error {
	switch {
	case errors.Is(err, biz.ErrInvalidArgument):
		return kerrors.BadRequest("INVALID_ARGUMENT", err.Error())
	case errors.Is(err, biz.ErrNotFound):
		return kerrors.NotFound("INVENTORY_NOT_FOUND", err.Error())
	case errors.Is(err, biz.ErrInsufficientStock):
		return kerrors.Conflict("INSUFFICIENT_STOCK", err.Error())
	case errors.Is(err, biz.ErrVersionConflict):
		return kerrors.Conflict("VERSION_CONFLICT", err.Error())
	case errors.Is(err, biz.ErrInvalidState):
		return kerrors.Conflict("INVALID_DEDUCTION_STATE", err.Error())
	default:
		return kerrors.InternalServer("DATABASE_ERROR", err.Error())
	}
}

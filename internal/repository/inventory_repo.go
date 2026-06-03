package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"inventory-service/internal/model"
)

// InventoryRepo 库存数据访问层
type InventoryRepo struct {
	db *gorm.DB
}

func NewInventoryRepo(db *gorm.DB) *InventoryRepo {
	return &InventoryRepo{db: db}
}

// GetByInvID 根据库存ID查询
func (r *InventoryRepo) GetByInvID(ctx context.Context, invID string) (*model.Inventory, error) {
	var inv model.Inventory
	err := r.db.WithContext(ctx).Where("inv_id = ?", invID).First(&inv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get inventory by inv_id failed: %w", err)
	}
	return &inv, nil
}

// Create 创建库存记录
func (r *InventoryRepo) Create(ctx context.Context, inv *model.Inventory) error {
	return r.db.WithContext(ctx).Create(inv).Error
}

// LockSqToLq 锁定可售库存到预锁库存（sq不动，直接加到lq）
// 使用乐观锁防止并发冲突
func (r *InventoryRepo) LockSqToLq(ctx context.Context, invID string, lockQuantity int) (bool, error) {
	// DB扣减时通过 where sq-lq > 0 控制最终sq的数量不能小于lq
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("inv_id = ? AND sq - lq >= ?", invID, lockQuantity).
		Update("lq", gorm.Expr("lq + ?", lockQuantity))

	if result.Error != nil {
		return false, fmt.Errorf("lock sq to lq failed: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ReleaseLq 释放预锁库存
func (r *InventoryRepo) ReleaseLq(ctx context.Context, invID string, releaseQuantity int) error {
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("inv_id = ? AND lq >= ?", invID, releaseQuantity).
		Update("lq", gorm.Expr("lq - ?", releaseQuantity))

	if result.Error != nil {
		return fmt.Errorf("release lq failed: %w", result.Error)
	}
	return nil
}

// MergeCommit 合并扣减DB（核心：sq - lq - delta > 0 防超卖）
func (r *InventoryRepo) MergeCommit(ctx context.Context, invID string, deltaQuantity int) (bool, error) {
	// 关键SQL：set sq = sq - delta where sq - lq - delta > 0
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("inv_id = ? AND sq - lq - ? > 0", invID, deltaQuantity).
		Updates(map[string]interface{}{
			"sq":         gorm.Expr("sq - ?", deltaQuantity),
			"lq":         gorm.Expr("lq - ?", deltaQuantity),
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return false, fmt.Errorf("merge commit failed: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// DirectDeduct 直接扣减DB（非热点兜底流程）
func (r *InventoryRepo) DirectDeduct(ctx context.Context, invID string, quantity int) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("inv_id = ? AND sq >= ?", invID, quantity).
		Updates(map[string]interface{}{
			"sq":         gorm.Expr("sq - ?", quantity),
			"wq":         gorm.Expr("wq + ?", quantity),
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return false, fmt.Errorf("direct deduct failed: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ConfirmDeduct 确认扣减(wq -> oq)
func (r *InventoryRepo) ConfirmDeduct(ctx context.Context, invID string, quantity int) error {
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("inv_id = ? AND wq >= ?", invID, quantity).
		Updates(map[string]interface{}{
			"wq":         gorm.Expr("wq - ?", quantity),
			"oq":         gorm.Expr("oq + ?", quantity),
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("confirm deduct failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("insufficient wq stock")
	}
	return nil
}

// CancelDeduct 取消扣减(wq -> sq)
func (r *InventoryRepo) CancelDeduct(ctx context.Context, invID string, quantity int) error {
	result := r.db.WithContext(ctx).Model(&model.Inventory{}).
		Where("inv_id = ? AND wq >= ?", invID, quantity).
		Updates(map[string]interface{}{
			"wq":         gorm.Expr("wq - ?", quantity),
			"sq":         gorm.Expr("sq + ?", quantity),
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("cancel deduct failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("insufficient wq stock")
	}
	return nil
}

// Upsert 插入或更新库存（幂等初始化）
func (r *InventoryRepo) Upsert(ctx context.Context, inv *model.Inventory) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "inv_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"sq", "updated_at"}),
	}).Create(inv).Error
}

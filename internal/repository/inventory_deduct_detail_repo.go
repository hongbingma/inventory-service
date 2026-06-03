package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"inventory-service/internal/model"
)

// DeductDetailRepo 扣减明细数据访问层
type DeductDetailRepo struct {
	db *gorm.DB
}

func NewDeductDetailRepo(db *gorm.DB) *DeductDetailRepo {
	return &DeductDetailRepo{db: db}
}

// Insert 插入扣减明细（利用唯一索引实现幂等）
func (r *DeductDetailRepo) Insert(ctx context.Context, detail *model.InventoryDeductDetail) error {
	err := r.db.WithContext(ctx).Create(detail).Error
	if err != nil {
		// 唯一键冲突视为幂等成功
		if isDuplicateKeyError(err) {
			return nil
		}
		return fmt.Errorf("insert deduct detail failed: %w", err)
	}
	return nil
}

// SumByLockOrderID 按锁库存单据ID汇总扣减数量（覆盖索引优化）
func (r *DeductDetailRepo) SumByLockOrderID(ctx context.Context, invID, lockOrderID string) (int64, error) {
	var sum int64
	err := r.db.WithContext(ctx).Model(&model.InventoryDeductDetail{}).
		Where("inv_id = ? AND lock_order_id = ? AND deduct_status = ?",
			invID, lockOrderID, model.DeductStatusPreDeduct).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&sum).Error
	if err != nil {
		return 0, fmt.Errorf("sum deduct detail by lock_order_id failed: %w", err)
	}
	return sum, nil
}

// GetByDeductID 根据扣减单据ID查询
func (r *DeductDetailRepo) GetByDeductID(ctx context.Context, deductID string) (*model.InventoryDeductDetail, error) {
	var detail model.InventoryDeductDetail
	err := r.db.WithContext(ctx).Where("deduct_id = ?", deductID).First(&detail).Error
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

// UpdateStatus 更新扣减状态
func (r *DeductDetailRepo) UpdateStatus(ctx context.Context, deductID string, status model.DeductStatus) error {
	return r.db.WithContext(ctx).Model(&model.InventoryDeductDetail{}).
		Where("deduct_id = ?", deductID).
		Update("deduct_status", status).Error
}

// isDuplicateKeyError 判断是否是唯一键冲突错误
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL唯一键冲突错误码: 1062
	return strings.Contains(err.Error(), "Duplicate entry") ||
		strings.Contains(err.Error(), "1062")
}

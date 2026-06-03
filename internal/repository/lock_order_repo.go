package repository

import (
	"context"

	"gorm.io/gorm"

	"inventory-service/internal/model"
)

// LockOrderRepo 锁库存单据数据访问层
type LockOrderRepo struct {
	db *gorm.DB
}

func NewLockOrderRepo(db *gorm.DB) *LockOrderRepo {
	return &LockOrderRepo{db: db}
}

// Insert 插入锁库存单据
func (r *LockOrderRepo) Insert(ctx context.Context, lockOrder *model.LockOrder) error {
	return r.db.WithContext(ctx).Create(lockOrder).Error
}

// GetByLockOrderID 根据锁库存单据ID查询
func (r *LockOrderRepo) GetByLockOrderID(ctx context.Context, lockOrderID string) (*model.LockOrder, error) {
	var lockOrder model.LockOrder
	err := r.db.WithContext(ctx).Where("lock_order_id = ?", lockOrderID).First(&lockOrder).Error
	if err != nil {
		return nil, err
	}
	return &lockOrder, nil
}

// UpdateStatus 更新锁库存单据状态
func (r *LockOrderRepo) UpdateStatus(ctx context.Context, lockOrderID string, status model.LockStatus) error {
	return r.db.WithContext(ctx).Model(&model.LockOrder{}).
		Where("lock_order_id = ?", lockOrderID).
		Update("lock_status", status).Error
}

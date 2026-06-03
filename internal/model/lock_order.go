package model

import "time"

// LockOrder 锁库存单据表模型
type LockOrder struct {
	ID           uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	LockOrderID  string     `gorm:"column:lock_order_id;type:varchar(64);not null;uniqueIndex:uk_lock_order_id" json:"lock_order_id"`
	InvID        string     `gorm:"column:inv_id;type:varchar(64);not null" json:"inv_id"`
	LockQuantity int        `gorm:"column:lock_quantity;not null" json:"lock_quantity"`
	BucketIndex  int        `gorm:"column:bucket_index;not null" json:"bucket_index"`
	LockStatus   LockStatus `gorm:"column:lock_status;type:tinyint;not null;default:1" json:"lock_status"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

func (LockOrder) TableName() string {
	return "lock_order"
}

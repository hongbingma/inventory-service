package model

import "time"

// Inventory 库存表模型
type Inventory struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	InvID     string    `gorm:"column:inv_id;type:varchar(64);not null;uniqueIndex:uk_inv_id" json:"inv_id"`
	Sq        int       `gorm:"column:sq;not null;default:0" json:"sq"`           // 可售库存
	Wq        int       `gorm:"column:wq;not null;default:0" json:"wq"`           // 预扣库存
	Oq        int       `gorm:"column:oq;not null;default:0" json:"oq"`           // 占用库存
	Lq        int       `gorm:"column:lq;not null;default:0" json:"lq"`           // 预锁库存
	Version   int64     `gorm:"column:version;not null;default:0" json:"version"` // 乐观锁版本号
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

func (Inventory) TableName() string {
	return "inventory"
}

// DeductStatus 扣减状态
type DeductStatus int8

const (
	DeductStatusPreDeduct DeductStatus = 1 // 预扣
	DeductStatusConfirmed DeductStatus = 2 // 已确认
	DeductStatusCancelled DeductStatus = 3 // 已取消
)

// DeductType 扣减类型
type DeductType int8

const (
	DeductTypeOrder      DeductType = 1 // 下单扣减
	DeductTypePayConfirm DeductType = 2 // 付款确认
	DeductTypeCancel     DeductType = 3 // 订单取消
)

// LockStatus 锁库存状态
type LockStatus int8

const (
	LockStatusActive   LockStatus = 1 // 锁定中
	LockStatusRecycled LockStatus = 2 // 已回收
)

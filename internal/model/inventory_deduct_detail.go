package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSONMap 用于存储 JSON 类型的扩展信息
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// InventoryDeductDetail 库存扣减明细表模型
type InventoryDeductDetail struct {
	ID           uint64       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DeductID     string       `gorm:"column:deduct_id;type:varchar(64);not null;uniqueIndex:uk_deduct_id" json:"deduct_id"`
	InvID        string       `gorm:"column:inv_id;type:varchar(64);not null" json:"inv_id"`
	LockOrderID  string       `gorm:"column:lock_order_id;type:varchar(64);not null" json:"lock_order_id"`
	OrderID      string       `gorm:"column:order_id;type:varchar(64);not null" json:"order_id"`
	Quantity     int          `gorm:"column:quantity;not null" json:"quantity"`
	DeductStatus DeductStatus `gorm:"column:deduct_status;type:tinyint;not null;default:1" json:"deduct_status"`
	DeductType   DeductType   `gorm:"column:deduct_type;type:tinyint;not null;default:1" json:"deduct_type"`
	ExtraInfo    JSONMap      `gorm:"column:extra_info;type:json" json:"extra_info"`
	CreatedAt    time.Time    `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
}

func (InventoryDeductDetail) TableName() string {
	return "inventory_deduct_detail"
}

package data

import (
	"context"
	"database/sql"
	"fmt"
)

type deductDetailRepo struct {
	db *sql.DB
}

// NewDeductDetailRepo creates a new deduct detail repository
func NewDeductDetailRepo(db *sql.DB) DeductDetailRepo {
	return &deductDetailRepo{db: db}
}

func (r *deductDetailRepo) CreateDeductDetail(deductID, invID, lockOrderID, orderID string, qty int32, deductType int32) error {
	_, err := r.db.ExecContext(
		context.Background(),
		"INSERT INTO inventory_deduct_detail (deduct_id, inv_id, lock_order_id, order_id, quantity, deduct_status, deduct_type, created_at) VALUES (?, ?, ?, ?, ?, 1, ?, NOW())",
		deductID, invID, lockOrderID, orderID, qty, deductType,
	)
	return err
}

func (r *deductDetailRepo) GetDeductDetail(deductID string) (*DeductDetail, error) {
	row := r.db.QueryRowContext(
		context.Background(),
		"SELECT id, deduct_id, inv_id, lock_order_id, order_id, quantity, deduct_status, deduct_type, extra_info, created_at FROM inventory_deduct_detail WHERE deduct_id = ?",
		deductID,
	)

	var dd DeductDetail
	err := row.Scan(&dd.ID, &dd.DeductID, &dd.InvID, &dd.LockOrderID, &dd.OrderID, &dd.Quantity, &dd.DeductStatus, &dd.DeductType, &dd.ExtraInfo, &dd.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("deduct detail not found: %s", deductID)
		}
		return nil, err
	}

	return &dd, nil
}

func (r *deductDetailRepo) UpdateDeductStatus(deductID string, status int32) error {
	_, err := r.db.ExecContext(
		context.Background(),
		"UPDATE inventory_deduct_detail SET deduct_status = ? WHERE deduct_id = ?",
		status, deductID,
	)
	return err
}

func (r *deductDetailRepo) GetDeductDetailsByOrderID(orderID string) ([]*DeductDetail, error) {
	rows, err := r.db.QueryContext(
		context.Background(),
		"SELECT id, deduct_id, inv_id, lock_order_id, order_id, quantity, deduct_status, deduct_type, extra_info, created_at FROM inventory_deduct_detail WHERE order_id = ?",
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deductDetails []*DeductDetail
	for rows.Next() {
		var dd DeductDetail
		err := rows.Scan(&dd.ID, &dd.DeductID, &dd.InvID, &dd.LockOrderID, &dd.OrderID, &dd.Quantity, &dd.DeductStatus, &dd.DeductType, &dd.ExtraInfo, &dd.CreatedAt)
		if err != nil {
			return nil, err
		}
		deductDetails = append(deductDetails, &dd)
	}

	return deductDetails, rows.Err()
}

func (r *deductDetailRepo) GetDeductDetailsByInvID(invID string) ([]*DeductDetail, error) {
	rows, err := r.db.QueryContext(
		context.Background(),
		"SELECT id, deduct_id, inv_id, lock_order_id, order_id, quantity, deduct_status, deduct_type, extra_info, created_at FROM inventory_deduct_detail WHERE inv_id = ?",
		invID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deductDetails []*DeductDetail
	for rows.Next() {
		var dd DeductDetail
		err := rows.Scan(&dd.ID, &dd.DeductID, &dd.InvID, &dd.LockOrderID, &dd.OrderID, &dd.Quantity, &dd.DeductStatus, &dd.DeductType, &dd.ExtraInfo, &dd.CreatedAt)
		if err != nil {
			return nil, err
		}
		deductDetails = append(deductDetails, &dd)
	}

	return deductDetails, rows.Err()
}

package biz

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type memoryRepo struct {
	mu         sync.Mutex
	inv        Inventory
	deductions map[string]Deduction
}

func newMemoryRepo(total int64) *memoryRepo {
	return &memoryRepo{inv: Inventory{SkuID: 1, Total: total, Available: total, Version: 1}, deductions: map[string]Deduction{}}
}
func (m *memoryRepo) CreateOrReplace(context.Context, int64, int64) (Inventory, error) {
	return Inventory{}, nil
}
func (m *memoryRepo) Get(context.Context, int64) (Inventory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inv, nil
}
func (m *memoryRepo) Deduct(_ context.Context, requestID string, skuID, quantity int64) (Inventory, Deduction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.deductions[requestID]; ok {
		return m.inv, d, nil
	}
	if m.inv.Available < quantity {
		return Inventory{}, Deduction{}, ErrInsufficientStock
	}
	m.inv.Available -= quantity
	m.inv.Locked += quantity
	m.inv.Version++
	d := Deduction{RequestID: requestID, SkuID: skuID, Quantity: quantity, Status: "LOCKED"}
	m.deductions[requestID] = d
	return m.inv, d, nil
}
func (m *memoryRepo) Release(_ context.Context, releaseRequestID, deductionRequestID string) (Inventory, Deduction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deductions[deductionRequestID]
	if !ok {
		return Inventory{}, Deduction{}, ErrNotFound
	}
	if d.Status == "LOCKED" {
		m.inv.Available += d.Quantity
		m.inv.Locked -= d.Quantity
		m.inv.Version++
		d.Status = "RELEASED"
		m.deductions[deductionRequestID] = d
	}
	return m.inv, d, nil
}
func (m *memoryRepo) Confirm(_ context.Context, confirmRequestID, deductionRequestID string) (Inventory, Deduction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deductions[deductionRequestID]
	if !ok {
		return Inventory{}, Deduction{}, ErrNotFound
	}
	if d.Status == "LOCKED" {
		m.inv.Locked -= d.Quantity
		m.inv.Sold += d.Quantity
		m.inv.Version++
		d.Status = "CONFIRMED"
		m.deductions[deductionRequestID] = d
	}
	return m.inv, d, nil
}
func (m *memoryRepo) Edit(context.Context, int64, int64, int64) (Inventory, error) {
	return Inventory{}, nil
}

func TestDeductIsIdempotent(t *testing.T) {
	uc := NewInventoryUsecase(newMemoryRepo(10))
	inv, _, err := uc.Deduct(context.Background(), "order-1", 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	inv, _, err = uc.Deduct(context.Background(), "order-1", 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Available != 7 || inv.Locked != 3 {
		t.Fatalf("unexpected inventory: %+v", inv)
	}
}

func TestDeductRejectsOversell(t *testing.T) {
	uc := NewInventoryUsecase(newMemoryRepo(2))
	_, _, err := uc.Deduct(context.Background(), "order-1", 1, 3)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("want insufficient stock, got %v", err)
	}
}

func TestReleaseReturnsLockedStock(t *testing.T) {
	uc := NewInventoryUsecase(newMemoryRepo(5))
	_, _, err := uc.Deduct(context.Background(), "order-1", 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	inv, ded, err := uc.Release(context.Background(), "cancel-1", "order-1")
	if err != nil {
		t.Fatal(err)
	}
	if ded.Status != "RELEASED" || inv.Available != 5 || inv.Locked != 0 {
		t.Fatalf("unexpected release result: inv=%+v ded=%+v", inv, ded)
	}
}

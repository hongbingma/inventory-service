package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"inventory-service/internal/config"
	"inventory-service/internal/model"
	redisMgr "inventory-service/internal/redis"
	"inventory-service/internal/repository"
)

// MergeCommitService 合并提交服务
type MergeCommitService struct {
	db         *gorm.DB
	cfg        *config.Config
	invRepo    *repository.InventoryRepo
	deductRepo *repository.DeductDetailRepo
	lockRepo   *repository.LockOrderRepo
	bucketMgr  *redisMgr.BucketManager
}

func NewMergeCommitService(
	db *gorm.DB,
	cfg *config.Config,
	invRepo *repository.InventoryRepo,
	deductRepo *repository.DeductDetailRepo,
	lockRepo *repository.LockOrderRepo,
	bucketMgr *redisMgr.BucketManager,
) *MergeCommitService {
	return &MergeCommitService{
		db:         db,
		cfg:        cfg,
		invRepo:    invRepo,
		deductRepo: deductRepo,
		lockRepo:   lockRepo,
		bucketMgr:  bucketMgr,
	}
}

// MergeCommit 合并提交（核心：扫描明细，一次提交DB）
func (s *MergeCommitService) MergeCommit(ctx context.Context, invID, lockOrderID string, bucketIndex int) error {
	// 1. 设置Redis扣减屏障（防并发超卖）
	barrierKey := lockOrderID
	if err := s.bucketMgr.SetBarrier(ctx, barrierKey, 60*time.Second); err != nil {
		return fmt.Errorf("set barrier failed: %w", err)
	}
	defer s.bucketMgr.RemoveBarrier(ctx, barrierKey)

	// 2. 失效Redis分桶，防止后续流量继续扣减此分桶
	if err := s.bucketMgr.DeleteBucket(ctx, invID, bucketIndex); err != nil {
		log.Printf("delete bucket failed: %v", err)
	}

	// 3. 扫描此分桶关联的所有扣减明细，计算实际扣减数量
	// 利用覆盖索引 inv_id, lock_order_id, quantity
	deductedSum, err := s.deductRepo.SumByLockOrderID(ctx, invID, lockOrderID)
	if err != nil {
		return fmt.Errorf("sum deduct detail failed: %w", err)
	}
	log.Printf("merge commit: invID=%s, lockOrderID=%s, deductedSum=%d", invID, lockOrderID, deductedSum)

	if deductedSum <= 0 {
		// 没有扣减明细，直接回收库存
		return s.recycleLockInventory(ctx, invID, lockOrderID)
	}

	// 4. 合并扣减DB（一次SQL提交）
	// set sq = sq - delta where sq - lq - delta > 0
	delta := int(deductedSum)
	success, err := s.invRepo.MergeCommit(ctx, invID, delta)
	if err != nil {
		return fmt.Errorf("merge commit db failed: %w", err)
	}

	if success {
		log.Printf("merge commit success: invID=%s, delta=%d", invID, delta)
		// 更新锁库存单据状态
		_ = s.lockRepo.UpdateStatus(ctx, lockOrderID, model.LockStatusRecycled)
	} else {
		log.Printf("merge commit failed: invID=%s, insufficient sq-lq", invID)
		// 合并提交失败，需要回补Redis库存
		_ = s.bucketMgr.InitBucket(ctx, invID, bucketIndex, delta, 30*time.Minute)
	}

	return nil
}

// recycleLockInventory 回收锁库存到可售状态
func (s *MergeCommitService) recycleLockInventory(ctx context.Context, invID, lockOrderID string) error {
	lockOrder, err := s.lockRepo.GetByLockOrderID(ctx, lockOrderID)
	if err != nil {
		return err
	}

	// 释放预锁库存
	if err := s.invRepo.ReleaseLq(ctx, invID, lockOrder.LockQuantity); err != nil {
		return err
	}

	// 更新锁库存单据状态
	return s.lockRepo.UpdateStatus(ctx, lockOrderID, model.LockStatusRecycled)
}

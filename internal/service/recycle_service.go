package service

import (
	"context"
	"log"

	"gorm.io/gorm"

	"inventory-service/internal/config"
	"inventory-service/internal/model"
	redisMgr "inventory-service/internal/redis"
	"inventory-service/internal/repository"
)

// RecycleService 库存回收服务
type RecycleService struct {
	db        *gorm.DB
	cfg       *config.Config
	invRepo   *repository.InventoryRepo
	lockRepo  *repository.LockOrderRepo
	bucketMgr *redisMgr.BucketManager
	mergeSvc  *MergeCommitService
}

func NewRecycleService(
	db *gorm.DB,
	cfg *config.Config,
	invRepo *repository.InventoryRepo,
	lockRepo *repository.LockOrderRepo,
	bucketMgr *redisMgr.BucketManager,
	mergeSvc *MergeCommitService,
) *RecycleService {
	return &RecycleService{
		db:        db,
		cfg:       cfg,
		invRepo:   invRepo,
		lockRepo:  lockRepo,
		bucketMgr: bucketMgr,
		mergeSvc:  mergeSvc,
	}
}

// Recycle 回收库存
// 场景：商家编辑库存、临界场景等
func (s *RecycleService) Recycle(ctx context.Context, invID string) error {
	// 查询所有锁定中的锁库存单据
	var lockOrders []model.LockOrder
	err := s.db.WithContext(ctx).Where("inv_id = ? AND lock_status = ?", invID, model.LockStatusActive).
		Find(&lockOrders).Error
	if err != nil {
		return err
	}

	for _, lockOrder := range lockOrders {
		log.Printf("recycling lock order: %s", lockOrder.LockOrderID)
		// 通过调用分桶合并提交来完成回收
		if err := s.mergeSvc.MergeCommit(ctx, invID, lockOrder.LockOrderID, lockOrder.BucketIndex); err != nil {
			log.Printf("recycle merge commit failed: %v", err)
			continue
		}
	}

	return nil
}

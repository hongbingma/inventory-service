package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory-service/internal/config"
	"inventory-service/internal/model"
	redisMgr "inventory-service/internal/redis"
	"inventory-service/internal/repository"
)

// LockInventoryService 锁库存服务
type LockInventoryService struct {
	db        *gorm.DB
	cfg       *config.Config
	invRepo   *repository.InventoryRepo
	lockRepo  *repository.LockOrderRepo
	bucketMgr *redisMgr.BucketManager
}

func NewLockInventoryService(
	db *gorm.DB,
	cfg *config.Config,
	invRepo *repository.InventoryRepo,
	lockRepo *repository.LockOrderRepo,
	bucketMgr *redisMgr.BucketManager,
) *LockInventoryService {
	return &LockInventoryService{
		db:        db,
		cfg:       cfg,
		invRepo:   invRepo,
		lockRepo:  lockRepo,
		bucketMgr: bucketMgr,
	}
}

// LockInventory 锁定库存到Redis分桶
// 返回锁库存单据ID和分桶索引
func (s *LockInventoryService) LockInventory(ctx context.Context, invID string) (string, int, error) {
	// 1. 查询库存记录
	inv, err := s.invRepo.GetByInvID(ctx, invID)
	if err != nil {
		return "", 0, fmt.Errorf("get inventory failed: %w", err)
	}
	if inv == nil {
		return "", 0, fmt.Errorf("inventory not found: %s", invID)
	}

	// 2. 计算锁定数量（按比例锁一部分到Redis）
	lockQuantity := int(float64(inv.Sq) * s.cfg.LockPercent)
	if lockQuantity <= 0 {
		return "", 0, fmt.Errorf("no available stock to lock")
	}

	// 3. 选择分桶索引
	bucketIndex := s.selectBucket(ctx, invID)

	// 4. 在DB事务中：锁定库存 + 创建锁库存单据
	lockOrderID := "lock_" + uuid.New().String()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 4.1 锁定DB库存（sq不动，加到lq）
		locked, err := s.invRepo.LockSqToLq(ctx, invID, lockQuantity)
		if err != nil {
			return err
		}
		if !locked {
			return fmt.Errorf("lock stock failed: insufficient sq")
		}

		// 4.2 创建锁库存单据
		lockOrder := &model.LockOrder{
			LockOrderID:  lockOrderID,
			InvID:        invID,
			LockQuantity: lockQuantity,
			BucketIndex:  bucketIndex,
			LockStatus:   model.LockStatusActive,
		}
		if err := s.lockRepo.Insert(ctx, lockOrder); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", 0, fmt.Errorf("lock inventory transaction failed: %w", err)
	}

	// 5. 初始化Redis分桶库存
	err = s.bucketMgr.InitBucket(ctx, invID, bucketIndex, lockQuantity, 30*time.Minute)
	if err != nil {
		// Redis初始化失败，需要回滚DB锁库存
		s.rollbackLock(ctx, invID, lockOrderID, lockQuantity)
		return "", 0, fmt.Errorf("init redis bucket failed: %w", err)
	}

	return lockOrderID, bucketIndex, nil
}

// selectBucket 选择Redis分桶（简单的取模选择）
func (s *LockInventoryService) selectBucket(ctx context.Context, invID string) int {
	// 可以改进为根据当前分桶负载动态选择，这里简化为随机选择
	return int(time.Now().UnixNano()) % s.cfg.BucketCount
}

// rollbackLock 回滚锁库存
func (s *LockInventoryService) rollbackLock(ctx context.Context, invID, lockOrderID string, quantity int) {
	_ = s.invRepo.ReleaseLq(ctx, invID, quantity)
	_ = s.lockRepo.UpdateStatus(ctx, lockOrderID, model.LockStatusRecycled)
}

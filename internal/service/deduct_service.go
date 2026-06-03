package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory-service/internal/config"
	"inventory-service/internal/model"
	redisMgr "inventory-service/internal/redis"
	"inventory-service/internal/repository"
)

// DeductService 下单扣减服务
type DeductService struct {
	db         *gorm.DB
	cfg        *config.Config
	invRepo    *repository.InventoryRepo
	deductRepo *repository.DeductDetailRepo
	lockRepo   *repository.LockOrderRepo
	bucketMgr  *redisMgr.BucketManager
	lockSvc    *LockInventoryService
	mergeSvc   *MergeCommitService
}

func NewDeductService(
	db *gorm.DB,
	cfg *config.Config,
	invRepo *repository.InventoryRepo,
	deductRepo *repository.DeductDetailRepo,
	lockRepo *repository.LockOrderRepo,
	bucketMgr *redisMgr.BucketManager,
	lockSvc *LockInventoryService,
	mergeSvc *MergeCommitService,
) *DeductService {
	return &DeductService{
		db:         db,
		cfg:        cfg,
		invRepo:    invRepo,
		deductRepo: deductRepo,
		lockRepo:   lockRepo,
		bucketMgr:  bucketMgr,
		lockSvc:    lockSvc,
		mergeSvc:   mergeSvc,
	}
}

// DeductRequest 扣减请求
type DeductRequest struct {
	InvID    string // 库存ID
	OrderID  string // 订单ID
	Quantity int    // 扣减数量
}

// DeductResponse 扣减响应
type DeductResponse struct {
	DeductID string // 扣减单据ID
	Success  bool   // 是否成功
	Message  string // 附加信息
}

// Deduct 下单扣减（核心流程）
func (s *DeductService) Deduct(ctx context.Context, req *DeductRequest) (*DeductResponse, error) {
	// 1. 尝试通过Redis分桶扣减（热点路径）
	resp, err := s.deductViaRedis(ctx, req)
	if err != nil {
		// 2. Redis扣减失败，降级走DB直接扣减（非热点路径）
		log.Printf("deduct via redis failed: %v, fallback to db direct deduct", err)
		return s.deductViaDB(ctx, req)
	}
	return resp, nil
}

// deductViaRedis Redis分桶扣减（热点路径）
func (s *DeductService) deductViaRedis(ctx context.Context, req *DeductRequest) (*DeductResponse, error) {
	// 1. 尝试获取可用的锁库存单据
	lockOrder, err := s.getAvailableLockOrder(ctx, req.InvID)
	if err != nil || lockOrder == nil {
		// 没有可用锁库存，自动触发锁库存
		lockOrderID, bucketIndex, lockErr := s.lockSvc.LockInventory(ctx, req.InvID)
		if lockErr != nil {
			return nil, fmt.Errorf("auto lock inventory failed: %w", lockErr)
		}
		lockOrder = &model.LockOrder{
			LockOrderID: lockOrderID,
			BucketIndex: bucketIndex,
		}
	}

	// 2. Redis分桶扣减计数
	remain, err := s.bucketMgr.DecrBucket(ctx, req.InvID, lockOrder.BucketIndex, req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("redis bucket decr failed: %w", err)
	}
	log.Printf("redis bucket decr success, remain: %d", remain)

	// 3. 插入DB扣减明细
	deductID := "deduct_" + uuid.New().String()
	detail := &model.InventoryDeductDetail{
		DeductID:     deductID,
		InvID:        req.InvID,
		LockOrderID:  lockOrder.LockOrderID,
		OrderID:      req.OrderID,
		Quantity:     req.Quantity,
		DeductStatus: model.DeductStatusPreDeduct,
		DeductType:   model.DeductTypeOrder,
	}

	if err := s.deductRepo.Insert(ctx, detail); err != nil {
		// 明细插入失败，回补Redis库存
		_, _ = s.bucketMgr.IncrBucket(ctx, req.InvID, lockOrder.BucketIndex, req.Quantity)
		return nil, fmt.Errorf("insert deduct detail failed: %w", err)
	}

	// 4. 触发延迟合并提交
	s.scheduleMergeCommit(ctx, req.InvID, lockOrder.LockOrderID, lockOrder.BucketIndex)

	return &DeductResponse{
		DeductID: deductID,
		Success:  true,
		Message:  "deduct via redis bucket success",
	}, nil
}

// deductViaDB 直接DB扣减（非热点兜底）
func (s *DeductService) deductViaDB(ctx context.Context, req *DeductRequest) (*DeductResponse, error) {
	success, err := s.invRepo.DirectDeduct(ctx, req.InvID, req.Quantity)
	if err != nil {
		return nil, fmt.Errorf("db direct deduct failed: %w", err)
	}
	if !success {
		return &DeductResponse{
			Success: false,
			Message: "insufficient stock",
		}, nil
	}

	// 插入扣减明细
	deductID := "deduct_" + uuid.New().String()
	detail := &model.InventoryDeductDetail{
		DeductID:     deductID,
		InvID:        req.InvID,
		LockOrderID:  "", // 非热点路径无锁库存单据
		OrderID:      req.OrderID,
		Quantity:     req.Quantity,
		DeductStatus: model.DeductStatusPreDeduct,
		DeductType:   model.DeductTypeOrder,
	}

	if err := s.deductRepo.Insert(ctx, detail); err != nil {
		return nil, fmt.Errorf("insert deduct detail failed: %w", err)
	}

	return &DeductResponse{
		DeductID: deductID,
		Success:  true,
		Message:  "deduct via db direct success",
	}, nil
}

// getAvailableLockOrder 获取可用的锁库存单据
func (s *DeductService) getAvailableLockOrder(ctx context.Context, invID string) (*model.LockOrder, error) {
	// 查询第一个锁定中的锁库存单据
	var lockOrder model.LockOrder
	err := s.db.WithContext(ctx).Where("inv_id = ? AND lock_status = ?", invID, model.LockStatusActive).
		First(&lockOrder).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &lockOrder, nil
}

// scheduleMergeCommit 调度合并提交
func (s *DeductService) scheduleMergeCommit(ctx context.Context, invID, lockOrderID string, bucketIndex int) {
	// 延迟执行合并提交
	time.AfterFunc(time.Duration(s.cfg.MergeDelayMs)*time.Millisecond, func() {
		_ = s.mergeSvc.MergeCommit(context.Background(), invID, lockOrderID, bucketIndex)
	})
}

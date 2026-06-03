package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"inventory-service/internal/config"
	"inventory-service/internal/model"
	redisMgr "inventory-service/internal/redis"
	"inventory-service/internal/repository"
	"inventory-service/internal/service"
)

func setupTest() (*gorm.DB, *redis.Client, func()) {
	// 连接数据库（测试环境）
	db, err := gorm.Open(mysql.Open("root:password@tcp(127.0.0.1:3306)/inventory_test?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// 自动迁移表结构
	db.AutoMigrate(&model.Inventory{}, &model.InventoryDeductDetail{}, &model.LockOrder{})

	// 连接Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
	})

	// 清理函数
	cleanup := func() {
		db.Exec("TRUNCATE TABLE inventory")
		db.Exec("TRUNCATE TABLE inventory_deduct_detail")
		db.Exec("TRUNCATE TABLE lock_order")
		redisClient.FlushDB(context.Background())
	}

	return db, redisClient, cleanup
}

// TestBasicDeductFlow 测试基本扣减流程
func TestBasicDeductFlow(t *testing.T) {
	db, redisClient, cleanup := setupTest()
	defer cleanup()

	cfg := &config.Config{
		BucketCount:  10,
		LockPercent:  0.5,
		MergeDelayMs: 200,
	}

	bucketMgr := redisMgr.NewBucketManager(redisClient, cfg.BucketCount)
	invRepo := repository.NewInventoryRepo(db)
	deductRepo := repository.NewDeductDetailRepo(db)
	lockRepo := repository.NewLockOrderRepo(db)

	mergeSvc := service.NewMergeCommitService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr)
	lockSvc := service.NewLockInventoryService(db, cfg, invRepo, lockRepo, bucketMgr)
	deductSvc := service.NewDeductService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr, lockSvc, mergeSvc)

	ctx := context.Background()

	// 1. 初始化库存：sq=100
	inv := &model.Inventory{
		InvID: "SKU001_WH001",
		Sq:    100,
	}
	err := invRepo.Create(ctx, inv)
	assert.NoError(t, err)

	// 2. 锁库存到Redis
	lockOrderID, bucketIndex, err := lockSvc.LockInventory(ctx, "SKU001_WH001")
	assert.NoError(t, err)
	assert.NotEmpty(t, lockOrderID)
	t.Logf("lock inventory success: lockOrderID=%s, bucketIndex=%d", lockOrderID, bucketIndex)

	// 3. 下单扣减（走Redis分桶）
	resp, err := deductSvc.Deduct(ctx, &service.DeductRequest{
		InvID:    "SKU001_WH001",
		OrderID:  "ORDER_001",
		Quantity: 5,
	})
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	t.Logf("deduct success: deductID=%s", resp.DeductID)

	// 4. 等待合并提交完成
	time.Sleep(500 * time.Millisecond)

	// 5. 验证库存数据
	updatedInv, err := invRepo.GetByInvID(ctx, "SKU001_WH001")
	assert.NoError(t, err)
	t.Logf("final inventory: sq=%d, wq=%d, lq=%d", updatedInv.Sq, updatedInv.Wq, updatedInv.Lq)
	// sq应该从100减到95（扣了5个）
	assert.Equal(t, 95, updatedInv.Sq)
}

// TestConcurrentDeduct 测试并发扣减
func TestConcurrentDeduct(t *testing.T) {
	db, redisClient, cleanup := setupTest()
	defer cleanup()

	cfg := &config.Config{
		BucketCount:  10,
		LockPercent:  0.8,
		MergeDelayMs: 500,
	}

	bucketMgr := redisMgr.NewBucketManager(redisClient, cfg.BucketCount)
	invRepo := repository.NewInventoryRepo(db)
	deductRepo := repository.NewDeductDetailRepo(db)
	lockRepo := repository.NewLockOrderRepo(db)

	mergeSvc := service.NewMergeCommitService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr)
	lockSvc := service.NewLockInventoryService(db, cfg, invRepo, lockRepo, bucketMgr)
	deductSvc := service.NewDeductService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr, lockSvc, mergeSvc)

	ctx := context.Background()

	// 初始化库存
	inv := &model.Inventory{
		InvID: "SKU002_WH001",
		Sq:    200,
	}
	err := invRepo.Create(ctx, inv)
	assert.NoError(t, err)

	// 锁库存
	_, _, err = lockSvc.LockInventory(ctx, "SKU002_WH001")
	assert.NoError(t, err)

	// 并发扣减100个请求，每个扣1件
	concurrency := 100
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := deductSvc.Deduct(ctx, &service.DeductRequest{
				InvID:    "SKU002_WH001",
				OrderID:  fmt.Sprintf("ORDER_%d", idx),
				Quantity: 1,
			})
			if err == nil && resp.Success {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	t.Logf("concurrent deduct result: success=%d", successCount)

	// 等待合并提交完成
	time.Sleep(1 * time.Second)

	// 验证最终库存
	updatedInv, err := invRepo.GetByInvID(ctx, "SKU002_WH001")
	assert.NoError(t, err)
	expectedSq := 200 - successCount
	t.Logf("final inventory: sq=%d, expected=%d, successCount=%d", updatedInv.Sq, expectedSq, successCount)
	assert.Equal(t, expectedSq, updatedInv.Sq, "库存应该准确扣减，不超卖不少卖")
}

// TestIdempotent 测试幂等性
func TestIdempotent(t *testing.T) {
	db, redisClient, cleanup := setupTest()
	defer cleanup()

	cfg := &config.Config{
		BucketCount:  10,
		LockPercent:  0.5,
		MergeDelayMs: 500,
	}

	bucketMgr := redisMgr.NewBucketManager(redisClient, cfg.BucketCount)
	invRepo := repository.NewInventoryRepo(db)
	deductRepo := repository.NewDeductDetailRepo(db)
	lockRepo := repository.NewLockOrderRepo(db)

	mergeSvc := service.NewMergeCommitService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr)
	lockSvc := service.NewLockInventoryService(db, cfg, invRepo, lockRepo, bucketMgr)
	deductSvc := service.NewDeductService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr, lockSvc, mergeSvc)

	ctx := context.Background()

	// 初始化库存
	inv := &model.Inventory{
		InvID: "SKU003_WH001",
		Sq:    10,
	}
	err := invRepo.Create(ctx, inv)
	assert.NoError(t, err)

	// 锁库存
	_, _, err = lockSvc.LockInventory(ctx, "SKU003_WH001")
	assert.NoError(t, err)

	// 第一次扣减
	resp1, err := deductSvc.Deduct(ctx, &service.DeductRequest{
		InvID:    "SKU003_WH001",
		OrderID:  "ORDER_IDEMPOTENT_001",
		Quantity: 2,
	})
	assert.NoError(t, err)
	assert.True(t, resp1.Success)
	t.Logf("first deduct success: deductID=%s", resp1.DeductID)

	// 第二次扣减（相同的deductID，应被幂等处理）
	// 注意：实际生产中由上游业务保证deductID唯一性，这里测试DB唯一索引幂等
	// 模拟重复插入
	detail := &model.InventoryDeductDetail{
		DeductID:     resp1.DeductID, // 使用相同的deductID
		InvID:        "SKU003_WH001",
		LockOrderID:  "lock_test",
		OrderID:      "ORDER_IDEMPOTENT_001",
		Quantity:     2,
		DeductStatus: model.DeductStatusPreDeduct,
		DeductType:   model.DeductTypeOrder,
	}
	err = deductRepo.Insert(ctx, detail)
	assert.NoError(t, err, "重复插入应被幂等处理，不报错")
}

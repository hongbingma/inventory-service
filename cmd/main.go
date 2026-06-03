package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	rds "github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"inventory-service/internal/config"
	"inventory-service/internal/redis"
	"inventory-service/internal/repository"
	"inventory-service/internal/service"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化数据库
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// 初始化Redis
	redisClient := rds.NewClient(&rds.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	// 初始化各层组件
	bucketMgr := redis.NewBucketManager(redisClient, cfg.BucketCount)

	invRepo := repository.NewInventoryRepo(db)
	deductRepo := repository.NewDeductDetailRepo(db)
	lockRepo := repository.NewLockOrderRepo(db)

	mergeSvc := service.NewMergeCommitService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr)
	lockSvc := service.NewLockInventoryService(db, cfg, invRepo, lockRepo, bucketMgr)
	deductSvc := service.NewDeductService(db, cfg, invRepo, deductRepo, lockRepo, bucketMgr, lockSvc, mergeSvc)
	recycleSvc := service.NewRecycleService(db, cfg, invRepo, lockRepo, bucketMgr, mergeSvc)

	// 初始化HTTP路由
	router := gin.Default()

	// 库存扣减接口
	router.POST("/api/v1/inventory/deduct", func(c *gin.Context) {
		var req service.DeductRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resp, err := deductSvc.Deduct(c.Request.Context(), &req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, resp)
	})

	// 库存回收接口
	router.POST("/api/v1/inventory/recycle", func(c *gin.Context) {
		var req struct {
			InvID string `json:"inv_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := recycleSvc.Recycle(c.Request.Context(), req.InvID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 启动HTTP服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}

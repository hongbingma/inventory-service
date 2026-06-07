package main

import (
	"log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	rds "github.com/go-redis/redis/v8"

	v1 "inventory-service/api/v1"
	"inventory-service/internal/config"
	"inventory-service/internal/data"
	"inventory-service/internal/service"
)

func main() {
	cfg := config.Load()

	// Initialize MySQL database
	db, err := data.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Initialize Redis
	redisClient := rds.NewClient(&rds.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	// Initialize repositories
	inventoryRepo := data.NewInventoryRepo(db)
	lockOrderRepo := data.NewLockOrderRepo(db)
	deductDetailRepo := data.NewDeductDetailRepo(db)

	// Initialize inventory service with repositories and redis
	inventorySvc := service.NewInventoryService(
		inventoryRepo,
		lockOrderRepo,
		deductDetailRepo,
		redisClient,
		cfg,
	)

	// Setup HTTP server with middleware
	httpSrv := http.NewServer(
		http.Middleware(
			recovery.Recovery(),
			logging.Server(),
		),
	)
	v1.RegisterInventoryServiceHTTPServer(httpSrv, inventorySvc)

	// Setup gRPC server with middleware
	grpcSrv := grpc.NewServer(
		grpc.Middleware(
			recovery.Recovery(),
			logging.Server(),
		),
	)
	v1.RegisterInventoryServiceServer(grpcSrv, inventorySvc)

	// Create and run Kratos application
	app := kratos.New(
		kratos.Name("inventory-service"),
		kratos.Version("v1.0.0"),
		kratos.Server(
			httpSrv,
			grpcSrv,
		),
	)

	if err := app.Run(); err != nil {
		log.Fatalf("app run error: %v", err)
	}
}

package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"inventory-service/internal/biz"
	"inventory-service/internal/data"
	"inventory-service/internal/server"
	"inventory-service/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport"
)

func main() {
	ctx := context.Background()
	dsn := getenv("DATABASE_DSN", "postgres://inventory:inventory@127.0.0.1:5432/inventory?sslmode=disable")
	db, err := data.NewDB(ctx, data.Config{
		DSN:             dsn,
		MaxOpenConns:    getenvInt("DB_MAX_OPEN_CONNS", 50),
		MaxIdleConns:    getenvInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: time.Duration(getenvInt("DB_CONN_MAX_LIFETIME_MINUTES", 30)) * time.Minute,
	})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	redisBuckets, err := data.NewRedisBucketStore(ctx, data.RedisConfig{
		Addr:             getenv("REDIS_ADDR", ""),
		Username:         getenv("REDIS_USERNAME", ""),
		Password:         getenv("REDIS_PASSWORD", ""),
		DB:               getenvInt("REDIS_DB", 0),
		BucketCount:      getenvInt("REDIS_BUCKET_COUNT", 16),
		BucketLockSize:   int64(getenvInt("REDIS_BUCKET_LOCK_SIZE", 100)),
		OperationTimeout: time.Duration(getenvInt("REDIS_OPERATION_TIMEOUT_MS", 50)) * time.Millisecond,
		BucketKeyTTL:     time.Duration(getenvInt("REDIS_BUCKET_TTL_MINUTES", 30)) * time.Minute,
	})
	if err != nil {
		log.Printf("redis bucket store disabled: %v", err)
	}
	defer redisBuckets.Close()

	repo := data.NewInventoryRepoWithRedis(db, redisBuckets)
	uc := biz.NewInventoryUsecase(repo)
	inventorySvc := service.NewInventoryService(uc)
	httpSrv := server.NewHTTPServer(server.HTTPConfig{Addr: getenv("HTTP_ADDR", ":8000")}, inventorySvc)
	grpcSrv := server.NewGRPCServer(server.GRPCConfig{Addr: getenv("GRPC_ADDR", ":9000")}, inventorySvc)
	app := kratos.New(kratos.Name("inventory-service"), kratos.Server([]transport.Server{httpSrv, grpcSrv}...))
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

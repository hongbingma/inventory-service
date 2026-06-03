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

	repo := data.NewInventoryRepo(db)
	uc := biz.NewInventoryUsecase(repo)
	httpSrv := server.NewHTTPServer(server.HTTPConfig{Addr: getenv("HTTP_ADDR", ":8000")}, uc)
	app := kratos.New(kratos.Name("inventory-service"), kratos.Server([]transport.Server{httpSrv}...))
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

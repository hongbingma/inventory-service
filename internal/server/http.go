package server

import (
	"time"

	inventoryv1 "inventory-service/api/inventory/v1"
	"inventory-service/internal/service"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type HTTPConfig struct {
	Addr string
}

func NewHTTPServer(cfg HTTPConfig, inventorySvc *service.InventoryService) *khttp.Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8000"
	}
	srv := khttp.NewServer(khttp.Address(cfg.Addr), khttp.Timeout(10*time.Second), khttp.Middleware(recovery.Recovery()))
	srv.Route("/").GET("/healthz", func(ctx khttp.Context) error {
		return ctx.Result(200, map[string]string{"status": "ok"})
	})
	inventoryv1.RegisterInventoryHTTPServer(srv, inventorySvc)
	return srv
}

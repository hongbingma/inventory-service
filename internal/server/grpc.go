package server

import (
	"time"

	inventoryv1 "inventory-service/api/inventory/v1"
	"inventory-service/internal/service"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

type GRPCConfig struct {
	Addr string
}

func NewGRPCServer(cfg GRPCConfig, inventorySvc *service.InventoryService) *kgrpc.Server {
	if cfg.Addr == "" {
		cfg.Addr = ":9000"
	}
	srv := kgrpc.NewServer(kgrpc.Address(cfg.Addr), kgrpc.Timeout(10*time.Second), kgrpc.Middleware(recovery.Recovery()))
	inventoryv1.RegisterInventoryServer(srv, inventorySvc)
	return srv
}

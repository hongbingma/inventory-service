package data

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

type Config struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func NewDB(ctx context.Context, cfg Config) (*sql.DB, error) {
	if cfg.Driver == "" {
		cfg.Driver = "postgres"
	}
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 50
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 10
	}
	if cfg.ConnMaxLifetime <= 0 {
		cfg.ConnMaxLifetime = 30 * time.Minute
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

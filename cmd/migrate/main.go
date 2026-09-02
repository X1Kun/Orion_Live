package main

import (
	"context"
	"database/sql"
	"os/signal"
	"syscall"

	"github.com/X1Kun/orion-live/internal/config"
	"github.com/X1Kun/orion-live/migrations"
	"github.com/X1Kun/orion-live/pkg/logger"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	cfg, err := config.LoadMigration()
	if err != nil {
		logger.Log.WithError(err).Fatal("invalid configuration")
	}
	logger.InitLogger("production")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	db, err := sql.Open("mysql", cfg.DSN(true))
	if err != nil {
		logger.Log.WithError(err).Fatal("open mysql for migrations")
	}
	defer db.Close()
	if err := migrations.Up(ctx, db); err != nil {
		logger.Log.WithError(err).Fatal("apply database migrations")
	}
	logger.Log.Info("database migrations are current")
}

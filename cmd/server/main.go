package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/X1Kun/orion-live/internal/config"
	"github.com/X1Kun/orion-live/internal/handler"
	"github.com/X1Kun/orion-live/internal/health"
	"github.com/X1Kun/orion-live/internal/repository"
	"github.com/X1Kun/orion-live/internal/router"
	"github.com/X1Kun/orion-live/internal/service"
	"github.com/X1Kun/orion-live/pkg/logger"
	mysqlclient "github.com/X1Kun/orion-live/pkg/mysql"
	rabbitclient "github.com/X1Kun/orion-live/pkg/rabbitmq"
	redisclient "github.com/X1Kun/orion-live/pkg/redis"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logger.Log.WithError(err).Fatal("invalid configuration")
	}
	logger.InitLogger(cfg.Environment)
	if cfg.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), cfg.DependencyInitTimeout)
	defer cancelStartup()

	db, err := mysqlclient.Open(startupCtx, cfg.MySQL)
	if err != nil {
		logger.Log.WithError(err).Fatal("initialize mysql")
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Log.WithError(err).Fatal("access mysql connection pool")
	}
	defer sqlDB.Close()

	redis, err := redisclient.Open(startupCtx, cfg.Redis)
	if err != nil {
		logger.Log.WithError(err).Fatal("initialize redis")
	}
	defer redis.Close()

	rabbitMQ, err := rabbitclient.Open(startupCtx, cfg.RabbitMQ)
	if err != nil {
		logger.Log.WithError(err).Fatal("initialize rabbitmq")
	}
	defer rabbitMQ.Close()

	checker := health.NewChecker(sqlDB, redis, rabbitMQ)
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, cfg.JWTSecret, cfg.AccessTokenTTL)
	engine := router.New(
		handler.NewUserHandler(userService),
		handler.NewHealthHandler(checker, time.Second),
		cfg.JWTSecret,
	)
	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Log.WithField("address", cfg.HTTPAddress).Info("api server started")
		serverErr <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		logger.Log.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Log.WithError(err).Fatal("api server stopped unexpectedly")
		}
	}

	checker.SetDraining()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.WithError(err).Error("graceful shutdown did not complete")
		_ = server.Close()
	}
	logger.Log.Info("api server stopped")
}

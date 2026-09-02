package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Environment           string
	HTTPAddress           string
	DependencyInitTimeout time.Duration
	HTTPShutdownTimeout   time.Duration
	JWTSecret             string
	AccessTokenTTL        time.Duration
	MySQL                 MySQL
	Redis                 Redis
	RabbitMQ              RabbitMQ
}

func Load() (Config, error) {
	mysqlConfig := loadMySQL()
	redisConfig, err := loadRedis()
	if err != nil {
		return Config{}, err
	}
	rabbitMQConfig := loadRabbitMQ()
	httpShutdownTimeout, err := envDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	dependencyInitTimeout, err := envDuration("DEPENDENCY_INIT_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := envDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:           env("APP_ENV", "development"),
		HTTPAddress:           env("HTTP_ADDRESS", ":8080"),
		DependencyInitTimeout: dependencyInitTimeout,
		HTTPShutdownTimeout:   httpShutdownTimeout,
		JWTSecret:             os.Getenv("JWT_SECRET_KEY"),
		AccessTokenTTL:        accessTokenTTL,
		MySQL:                 mysqlConfig,
		Redis:                 redisConfig,
		RabbitMQ:              rabbitMQConfig,
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if err := validateRequired(map[string]string{
		"DB_USER":           c.MySQL.User,
		"DB_PASSWORD":       c.MySQL.Password,
		"DB_NAME":           c.MySQL.Database,
		"JWT_SECRET_KEY":    c.JWTSecret,
		"REDIS_PASSWORD":    c.Redis.Password,
		"RABBITMQ_USER":     c.RabbitMQ.User,
		"RABBITMQ_PASSWORD": c.RabbitMQ.Password,
	}); err != nil {
		return err
	}
	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET_KEY must contain at least 32 characters")
	}
	if c.HTTPShutdownTimeout <= 0 || c.DependencyInitTimeout <= 0 || c.AccessTokenTTL <= 0 {
		return errors.New("configured durations must be positive")
	}
	if c.Redis.Database < 0 {
		return errors.New("REDIS_DB must not be negative")
	}
	return nil
}

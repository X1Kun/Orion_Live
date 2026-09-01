package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddress     string
	ShutdownTimeout time.Duration
	StartupTimeout  time.Duration
	JWTSecret       string
	AccessTokenTTL  time.Duration
	MySQL           MySQL
	Redis           Redis
	RabbitMQ        RabbitMQ
}

type MySQL struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type Redis struct {
	Host     string
	Port     string
	Password string
	Database int
}

type RabbitMQ struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

func Load() (Config, error) {
	mysqlConfig, err := LoadMySQL()
	if err != nil {
		return Config{}, err
	}
	redisDB, err := envInt("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := envDuration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	startupTimeout, err := envDuration("STARTUP_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	accessTokenTTL, err := envDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:     env("APP_ENV", "development"),
		HTTPAddress:     env("HTTP_ADDRESS", ":8080"),
		ShutdownTimeout: shutdownTimeout,
		StartupTimeout:  startupTimeout,
		JWTSecret:       os.Getenv("JWT_SECRET_KEY"),
		AccessTokenTTL:  accessTokenTTL,
		MySQL:           mysqlConfig,
		Redis: Redis{
			Host:     env("REDIS_HOST", "127.0.0.1"),
			Port:     env("REDIS_PORT", "6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			Database: redisDB,
		},
		RabbitMQ: RabbitMQ{
			Host:     env("RABBITMQ_HOST", "127.0.0.1"),
			Port:     env("RABBITMQ_PORT", "5672"),
			User:     os.Getenv("RABBITMQ_USER"),
			Password: os.Getenv("RABBITMQ_PASSWORD"),
			VHost:    env("RABBITMQ_VHOST", "/"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadMySQL() (MySQL, error) {
	cfg := MySQL{
		Host:     env("DB_HOST", "127.0.0.1"),
		Port:     env("DB_PORT", "3306"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
	}
	for name, value := range map[string]string{
		"DB_USER": cfg.User, "DB_PASSWORD": cfg.Password, "DB_NAME": cfg.Database,
	} {
		if value == "" {
			return MySQL{}, fmt.Errorf("missing required environment variable: %s", name)
		}
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var missing []string
	for name, value := range map[string]string{
		"DB_USER":           c.MySQL.User,
		"DB_PASSWORD":       c.MySQL.Password,
		"DB_NAME":           c.MySQL.Database,
		"JWT_SECRET_KEY":    c.JWTSecret,
		"REDIS_PASSWORD":    c.Redis.Password,
		"RABBITMQ_USER":     c.RabbitMQ.User,
		"RABBITMQ_PASSWORD": c.RabbitMQ.Password,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing required environment variables: %v", missing)
	}
	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET_KEY must contain at least 32 characters")
	}
	if c.ShutdownTimeout <= 0 || c.StartupTimeout <= 0 || c.AccessTokenTTL <= 0 {
		return errors.New("configured durations must be positive")
	}
	if c.Redis.Database < 0 {
		return errors.New("REDIS_DB must not be negative")
	}
	return nil
}

func (c MySQL) DSN(multiStatements bool) string {
	values := url.Values{
		"charset":   []string{"utf8mb4"},
		"parseTime": []string{"true"},
		"loc":       []string{"UTC"},
	}
	if multiStatements {
		values.Set("multiStatements", "true")
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", c.User, c.Password, net.JoinHostPort(c.Host, c.Port), c.Database, values.Encode())
}

func (c Redis) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}

func (c RabbitMQ) URL() string {
	u := &url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   c.VHost,
	}
	return u.String()
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}
	return parsed, nil
}

func envInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return parsed, nil
}

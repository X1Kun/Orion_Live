package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"
)

type MySQL struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func loadMySQL() (MySQL, error) {
	cfg := loadMySQLConnection()
	maxOpenConns, err := envInt("DB_MAX_OPEN_CONNS", 30)
	if err != nil {
		return MySQL{}, err
	}
	maxIdleConns, err := envInt("DB_MAX_IDLE_CONNS", 10)
	if err != nil {
		return MySQL{}, err
	}
	connMaxLifetime, err := envDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return MySQL{}, err
	}

	cfg.MaxOpenConns = maxOpenConns
	cfg.MaxIdleConns = maxIdleConns
	cfg.ConnMaxLifetime = connMaxLifetime
	return cfg, nil
}

func loadMySQLConnection() MySQL {
	return MySQL{
		Host:     env("DB_HOST", "127.0.0.1"),
		Port:     env("DB_PORT", "3306"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
	}
}

func LoadMigration() (MySQL, error) {
	cfg := loadMySQLConnection()
	if err := validateRequired(map[string]string{
		"DB_USER": cfg.User, "DB_PASSWORD": cfg.Password, "DB_NAME": cfg.Database,
	}); err != nil {
		return MySQL{}, err
	}
	return cfg, nil
}

func (c MySQL) validate() error {
	if c.MaxOpenConns <= 0 {
		return errors.New("DB_MAX_OPEN_CONNS must be positive")
	}
	if c.MaxIdleConns < 0 || c.MaxIdleConns > c.MaxOpenConns {
		return errors.New("DB_MAX_IDLE_CONNS must be between 0 and DB_MAX_OPEN_CONNS")
	}
	if c.ConnMaxLifetime <= 0 {
		return errors.New("DB_CONN_MAX_LIFETIME must be positive")
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

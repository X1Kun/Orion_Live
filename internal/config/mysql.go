package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

type MySQL struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func loadMySQL() MySQL {
	return MySQL{
		Host:     env("DB_HOST", "127.0.0.1"),
		Port:     env("DB_PORT", "3306"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
	}
}

func LoadMigration() (MySQL, error) {
	cfg := loadMySQL()
	if err := validateRequired(map[string]string{
		"DB_USER": cfg.User, "DB_PASSWORD": cfg.Password, "DB_NAME": cfg.Database,
	}); err != nil {
		return MySQL{}, err
	}
	return cfg, nil
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

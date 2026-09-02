package config

import (
	"net"
	"os"
)

type Redis struct {
	Host     string
	Port     string
	Password string
	Database int
}

func loadRedis() (Redis, error) {
	database, err := envInt("REDIS_DB", 0)
	if err != nil {
		return Redis{}, err
	}
	cfg := Redis{
		Host:     env("REDIS_HOST", "127.0.0.1"),
		Port:     env("REDIS_PORT", "6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		Database: database,
	}
	return cfg, nil
}

func (c Redis) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}

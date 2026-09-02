package config

import (
	"net"
	"net/url"
	"os"
)

type RabbitMQ struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

func loadRabbitMQ() RabbitMQ {
	return RabbitMQ{
		Host:     env("RABBITMQ_HOST", "127.0.0.1"),
		Port:     env("RABBITMQ_PORT", "5672"),
		User:     os.Getenv("RABBITMQ_USER"),
		Password: os.Getenv("RABBITMQ_PASSWORD"),
		VHost:    env("RABBITMQ_VHOST", "/"),
	}
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

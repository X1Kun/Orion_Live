package rabbitmq

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
)

// 初始化RabbitMQ连接：1、从.env文件中加载环境变量 2、构建 url 3、打开RabbitMQ连接
func InitRabbitMQ() (*amqp.Connection, error) {
	_ = godotenv.Load()

	user := os.Getenv("RABBITMQ_USER")
	if user == "" {
		user = "guest"
	}
	password := os.Getenv("RABBITMQ_PASSWORD")
	if password == "" {
		password = "guest"
	}
	host := os.Getenv("RABBITMQ_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("RABBITMQ_PORT")
	if port == "" {
		port = "5672"
	}
	// AMQP 协议规范
	url := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, password, host, port)
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("无法连接到RabbitMQ at %s: %w", host, err)
	}
	return conn, nil
}

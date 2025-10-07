package rabbitmq

import (
	"github.com/streadway/amqp"
)

// InitRabbitMQ 初始化RabbitMQ连接
func InitRabbitMQ() (*amqp.Connection, error) {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		return nil, err
	}
	return conn, nil
}

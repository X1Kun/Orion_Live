package rabbitmq

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/X1Kun/orion-live/internal/config"
	"github.com/streadway/amqp"
)

func Open(ctx context.Context, cfg config.RabbitMQ) (*amqp.Connection, error) {
	conn, err := amqp.DialConfig(cfg.URL(), amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
		Dial: func(network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, address)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open rabbitmq connection: %w", err)
	}
	return conn, nil
}

package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

// 初始化Redis客户端：1、从.env文件中加载环境变量 2、创建redis链接 3、验证连接
func InitRedis() (*redis.Client, error) {
	// 加载 .env 文件。如果文件不存在，它会静默失败，程序会继续尝试从系统环境变量中读取。
	// 这使得代码在本地开发(有.env)和生产环境(直接设置环境变量)都能工作。
	_ = godotenv.Load()

	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}
	addr := fmt.Sprintf("%s:%s", host, port)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("无法连接到Redis at %s: %w", addr, err)
	}
	return rdb, nil
}

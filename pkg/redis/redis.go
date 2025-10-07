package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

var Ctx = context.Background()

// InitRedis 初始化Redis客户端
func InitRedis() (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", //redis运行地址
		Password: "",
		DB:       0, // 使用默认编号为0的数据库
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	// 如果到时间，自动关闭后台监视计时器的goroutine
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return rdb, nil
}

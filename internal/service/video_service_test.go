package service

import (
	"Orion_Live/internal/repository"
	"fmt"
	"testing"

	"Orion_Live/pkg/redis"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB and service for benchmark
func setupBenchmark() VideoService {
	// 在测试中，我们也需要一个真实的数据库连接
	dsn := "root:zhengxikun@tcp(127.0.0.1:3306)/orion_live?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // 👈 开启日志
	})
	if err != nil {
		panic(fmt.Sprintf("无法连接到数据库: %v", err))
	}

	redisClient, err := redis.InitRedis()
	if err != nil {
		panic(fmt.Sprintf("无法连接到Redis: %v", err))
	}

	videoRepo := repository.NewVideoRepository(db, redisClient)
	videoService := NewVideoService(videoRepo) // 假设MQ暂时不用

	return videoService
}

// BenchmarkGetVideoByID_CacheBreakdown 是我们的“攻城炮”
// 函数名必须以 Benchmark 开头
func BenchmarkGetVideoByID_CacheBreakdown(b *testing.B) {
	videoService := setupBenchmark()

	// 我们要攻击的目标视频ID
	targetVideoID := uint64(1)

	// 模拟缓存失效：在压测开始前，手动删除Redis缓存（如果存在的话）
	// (我们现在还没有缓存逻辑，所以这步暂时不需要)

	b.ResetTimer() // 重置计时器，忽略前面的准备时间

	// b.N 是由 testing 框架决定的一个巨大数字，代表执行次数
	// 我们用 b.RunParallel 来模拟高并发
	b.RunParallel(func(pb *testing.PB) {
		// 每个 goroutine 都会进入这个循环
		for pb.Next() {
			// 在这里，成百上千个goroutine会同时调用GetVideoByID
			_, err := videoService.GetVideoByID(targetVideoID)
			if err != nil {
				b.Errorf("GetVideoByID failed: %v", err)
			}
		}
	})
}

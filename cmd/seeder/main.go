// cmd/seeder/main.go

package main

import (
	"Orion_Live/internal/model" // 👈 确保路径正确
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/go-faker/faker/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func main() {
	fmt.Println("🚀 开始填充测试数据...")

	// --- 1. 连接数据库 ---
	// 注意：这里的DSN需要和你server/main.go中的保持一致
	dsn := "root:zhengxikun@tcp(127.0.0.1:3306)/orion_live?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ 无法连接到数据库: %v", err)
	}
	fmt.Println("✅ 数据库连接成功!")

	// --- 2. 清理旧数据 (可选，但推荐) ---
	fmt.Println("🧹 正在清理旧数据...")
	// 为了确保每次填充都是干净的，我们可以先删除旧表再重建
	// 注意：这将删除所有数据！
	db.Migrator().DropTable(&model.Comment{}, &model.Like{}, &model.Video{}, &model.User{})
	fmt.Println("✅ 旧表删除成功!")

	// 重新迁移，创建新表
	db.AutoMigrate(&model.User{}, &model.Video{}, &model.Like{}, &model.Comment{})
	fmt.Println("✅ 数据库迁移成功!")

	// --- 3. 创建用户 ---
	fmt.Println("👥 正在创建用户...")
	userCount := 100
	for i := 0; i < userCount; i++ {
		// 使用faker生成随机用户名
		username := faker.Username()

		// 为所有用户设置一个简单的默认密码 "password"
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("❌ 密码加密失败: %v", err)
		}

		user := model.User{
			Username: username,
			Password: string(hashedPassword),
		}
		db.Create(&user)
	}
	fmt.Printf("✅ 成功创建 %d 个用户!\n", userCount)

	// --- 4. 创建视频 ---
	fmt.Println("🎬 正在创建视频...")
	videoCount := 500
	// 初始化随机数种子，确保每次运行生成的随机作者不同
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < videoCount; i++ {
		video := model.Video{
			// 从已创建的100个用户中，随机选择一个作为作者
			// rand.Intn(userCount) 会生成 [0, 99] 之间的随机数, +1 后变为 [1, 100]
			AuthorID:    uint64(rand.Intn(userCount) + 1),
			Title:       faker.Sentence(),  // 生成一个随机的句子作为标题
			Description: faker.Paragraph(), // 生成一个随机的段落作为简介
			VideoURL:    "https://test.com/video.mp4",
			CoverURL:    "https://test.com/cover.jpg",
		}
		db.Create(&video)
	}
	fmt.Printf("✅ 成功创建 %d 个视频!\n", videoCount)

	// --- 5. (可选) 创建随机点赞 ---
	fmt.Println("👍 正在创建随机点赞...")
	likeCount := 1000
	for i := 0; i < likeCount; i++ {
		like := model.Like{
			UserID:  uint64(rand.Intn(userCount) + 1),
			VideoID: uint64(rand.Intn(videoCount) + 1),
		}
		// 使用GORM的 OnConflict 来避免因为重复点赞而报错
		// 这会尝试插入，如果因为唯一键冲突失败，就什么都不做
		db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
			DoNothing: true,
		}).Create(&like)
	}
	fmt.Printf("✅ 成功创建(或尝试创建) %d 个随机点赞!\n", likeCount)
	// 注意：这里的点赞数还没有同步到videos表的like_count字段，这是一个很好的练习！
	// 你可以尝试写一段SQL，在seeder的最后，去批量更新所有视频的点赞数。

	fmt.Println("🎉🎉🎉 所有测试数据填充完毕! 🎉🎉🎉")
}

package main

import (
	"Orion_Live/internal/data"
	"Orion_Live/internal/handler"
	"Orion_Live/internal/model"
	"Orion_Live/internal/repository"
	"Orion_Live/internal/router"
	"Orion_Live/internal/service"
	"Orion_Live/pkg/logger"
	"Orion_Live/pkg/rabbitmq"
	"Orion_Live/pkg/redis"
	"log"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 加载.env文件
	err := godotenv.Load()
	if err != nil {
		log.Fatalf(".env文件加载失败")
	}
	// 初始化logger
	logger.InitLogger()

	// 初始化Redis
	redisClient, err := redis.InitRedis()
	if err != nil {
		logger.Log.Fatalf("无法连接到Redis: %v", err)
	}
	logger.Log.Info("Redis连接成功")

	// 初始化RabbitMQ
	rabbitMQConn, err := rabbitmq.InitRabbitMQ()
	if err != nil {
		logger.Log.Fatalf("无法连接到RabbitMQ: %v", err)
	}
	defer rabbitMQConn.Close() // 确保程序退出时关闭连接
	logger.Log.Info("RabbitMQ连接成功")

	// 数据源名称，用户名:密码@网络协议(地址:端口号)/数据库名?charset=字符集&parseTime=是否解析时间&loc=时区
	dsn := "root:zhengxikun@tcp(127.0.0.1:3306)/orion_live?charset=utf8mb4&parseTime=True&loc=Local"
	// 这个mysql包是gorm的第三方承包商，mysql.Open()后还是只能执行原始SQL语句，gorm.Open()后可以执行gorm的简化语句，但要注意性能
	// db.Debug()/db.Raw()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Log.Fatalf("无法连接到数据库: %v", err)
	}
	logger.Log.Info("数据库连接成功")
	// db.AutoMigrate(),没有这个表就创建,没有属性列则创建列,没有约束则增加约束;不会主动删除和修改
	err = db.AutoMigrate(&model.User{}, &model.Video{}, &model.Like{}, &model.Comment{})
	if err != nil {
		logger.Log.Fatalf("数据库迁移失败: %v", err)
	}
	// 防止程序每次重启，都尝试去重复创建同一个索引
	// if !db.Migrator().HasIndex(&model.Like{}, "idx_user_video") {
	// 	db.Migrator().CreateIndex(&model.Like{}, "idx_user_video")
	// }
	logger.Log.Info("数据库迁移成功")

	userRepo := repository.NewUserRepository(db)
	videoRepo := repository.NewVideoRepository(db, redisClient)
	commentRepo := repository.NewCommentRepository(db)

	uow := data.NewUnitOfWork(db, videoRepo, commentRepo)

	userService := service.NewUserService(userRepo)
	videoService := service.NewVideoService(videoRepo)
	likeService := service.NewLikeService(videoRepo, rabbitMQConn)
	commentService := service.NewCommentService(commentRepo, videoRepo, uow, redisClient, rabbitMQConn)

	userHandler := handler.NewUserHandler(userService)
	videoHandler := handler.NewVideoHandler(videoService)
	likeHandler := handler.NewLikeHandler(likeService)
	commentHandler := handler.NewCommentHandler(commentService, commentRepo, videoRepo)

	r := router.SetupRouter(userHandler, videoHandler, likeHandler, commentHandler)
	logger.Log.Println("服务器将在: 8080端口启动")

	if err := r.Run(":8080"); err != nil {
		logger.Log.Fatalf("服务器启动失败: %v", err)
	}
	logger.Log.Println("服务器成功在: 8080端口启动")
}

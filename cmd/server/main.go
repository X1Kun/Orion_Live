package main

import (
	"Orion_Live/internal/data"
	"Orion_Live/internal/handler"
	"Orion_Live/internal/model"
	"Orion_Live/internal/repository"
	"Orion_Live/internal/router"
	"Orion_Live/internal/service"
	"Orion_Live/internal/websocket"
	"Orion_Live/pkg/logger"
	my "Orion_Live/pkg/mysql"
	"Orion_Live/pkg/rabbitmq"
	"Orion_Live/pkg/redis"
	"log"
)

func main() {
	// 初始化logger (logger通常最先初始化)
	logger.InitLogger()

	// 初始化数据库
	db, err := my.InitMySQL()
	if err != nil {
		log.Fatalf("无法初始化数据库: %v", err)
	}
	// 获取通用数据库对象并延迟关闭
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("无法获取底层的sql.DB: %v", err)
	}
	defer sqlDB.Close()
	logger.Log.Info("数据库连接成功")

	// 初始化Redis
	redisClient, err := redis.InitRedis()
	if err != nil {
		log.Fatalf("无法初始化Redis: %v", err)
	}
	defer redisClient.Close()
	logger.Log.Info("Redis连接成功")

	// 初始化RabbitMQ
	rabbitMQConn, err := rabbitmq.InitRabbitMQ()
	if err != nil {
		log.Fatalf("无法初始化RabbitMQ: %v", err)
	}
	defer rabbitMQConn.Close()
	logger.Log.Info("RabbitMQ连接成功")

	// db.AutoMigrate(),没有这个表就创建,没有属性列则创建列,没有约束则增加约束;不会主动删除和修改
	err = db.AutoMigrate(&model.User{}, &model.Video{}, &model.Like{}, &model.Comment{})
	if err != nil {
		logger.Log.Fatalf("数据库迁移失败: %v", err)
	}
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
	hub := websocket.NewHub()
	go hub.Run()
	wsHandler := handler.NewWebSocketHandler(hub, videoService)

	r := router.SetupRouter(userHandler, videoHandler, likeHandler, commentHandler, *wsHandler)
	logger.Log.Println("服务器将在: 8080端口启动")

	if err := r.Run(":8080"); err != nil {
		logger.Log.Fatalf("服务器启动失败: %v", err)
	}
	logger.Log.Println("服务器成功在: 8080端口启动")
}

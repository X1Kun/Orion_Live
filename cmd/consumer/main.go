package main

import (
	"Orion_Live/internal/data"
	"Orion_Live/internal/model"
	"Orion_Live/internal/repository"
	"Orion_Live/pkg/logger"
	"Orion_Live/pkg/rabbitmq"
	"encoding/json"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/streadway/amqp"
	gorm_mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	QueueGoldenComment = "orion.golden_comment.queue"
	QueueLike          = "orion.like.queue"
)

// like消息，只需要userID，videoID 和 “赞”/“取赞”
type LikeMessage struct {
	UserID  uint64 `json:"user_id"`
	VideoID uint64 `json:"video_id"`
	Action  string `json:"action"`
}

type GoldenCommentMessage struct {
	UserID  uint64 `json:"user_id"`
	VideoID uint64 `json:"video_id"`
	Content string `json:"content"`
}

// 消费者进程：连接mysql，rabbitMQ，利用mq和likeRepo进行mysql的持久化存储
func main() {
	logger.InitLogger()

	// 连接数据库
	dsn := "root:zhengxikun@tcp(127.0.0.1:3306)/orion_live?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(gorm_mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Log.Fatalf("消费者无法连接到数据库: %v", err)
	}
	// 连接RabbitMQ
	rabbitMQConn, err := rabbitmq.InitRabbitMQ()
	if err != nil {
		logger.Log.Fatalf("消费者无法连接到RabbitMQ: %v", err)
	}
	defer rabbitMQConn.Close()

	// likeRepo绑定mysql库
	likeRepo := repository.NewLikeRepository(db)
	videoRepo := repository.NewVideoRepository(db, nil)
	commentRepo := repository.NewCommentRepository(db)
	uow := data.NewUnitOfWork(db, videoRepo, commentRepo)
	// 开始消费消息
	consumeLikes(rabbitMQConn, db, likeRepo, videoRepo)
	consumeGoldenComments(rabbitMQConn, db, commentRepo, uow)
}

// like消息队列消费者：1、通过mq的TCP连接创建channel 2、通过ch注册消费者 3、利用无缓冲通道持续消费like消息 4、处理消息，repo负责增/删like关系，并对mq中的消息进行安全管理
func consumeLikes(conn *amqp.Connection, db *gorm.DB, repo repository.LikeRepository, videoRepo repository.VideoRepository) {
	ch, err := conn.Channel()
	if err != nil {
		logger.Log.Fatalf("无法打开Channel: %v", err)
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		QueueLike, // queue
		"",        // consumer
		false,     // auto-ack: 为简单起见，我们先用自动确认
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		logger.Log.Fatalf("无法注册点赞消费者: %v", err)
	}
	// 创建一个没有任何缓冲的bool类型通道
	forever := make(chan bool)

	go func() {
		// msgs不是切片，而是通道channel，如果通道为空不会结束循环，而会“阻塞”
		for d := range msgs {
			logCtx := logger.Log.WithField("body", string(d.Body)).WithField("redelivered", d.Redelivered)
			// logCtx := logger.Log.WithField("body", string(d.Body))
			logCtx.Info("收到一条点赞消息")

			var msg LikeMessage
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				logCtx.WithError(err).Error("消息JSON解析失败")
				// 对于无法解析的“坏消息”，应该通知mq处理失败，并直接删除
				d.Nack(false, false)
				continue // 处理下一条
			}
			// db.Transaction事务操作，必须由原始的、全局的数据库连接池(db)来发起，并且tx是一次性的
			// 如果返回error，gorm会向数据库发送ROLLBACK指令，在tx上的操作，会被撤销
			// 如果返回nil，gorm会向数据库发送COMMIT指令，在tx上的操作，会被写入数据库
			err := db.Transaction(func(tx *gorm.DB) error {
				// 在事务中，我们需要使用临时的、绑定到这个事务(tx)的repository实例
				txLikeRepo := repository.NewLikeRepository(tx)
				txVideoRepo := repository.NewVideoRepository(tx, nil) // 事务中不操作Redis，所以rdb传nil

				if msg.Action == "like" {
					like := &model.Like{UserID: msg.UserID, VideoID: msg.VideoID}
					if err := txLikeRepo.Create(like); err != nil {
						return err // 事务中，返回任何error都会导致回滚
					}
					if err := txVideoRepo.IncrementLikeCount(msg.VideoID); err != nil {
						return err
					}
				} else if msg.Action == "unlike" {
					if err := txLikeRepo.Delete(msg.UserID, msg.VideoID); err != nil {
						return err
					}
					if err := txVideoRepo.DecrementLikeCount(msg.VideoID); err != nil {
						return err
					}
				}
				return nil // 事务成功，返回nil，GORM会自动提交(Commit)
			})
			opErr := err
			// 根据数据库操作的结果，来决定如何“确认”消息
			if opErr != nil {
				var mysqlErr *mysql.MySQLError
				// 用errors.As来检查错误的“根”是不是一个MySQLError
				if errors.As(opErr, &mysqlErr) && mysqlErr.Number == 1062 {
					// 错误号 1062 就是 "Duplicate entry"
					logCtx.WithError(opErr).Warn("处理消息时出现重复键错误，可能是一次重复消费，消息将被确认为成功。")
					// 这不是一个需要重试的错误，直接Ack掉
					d.Ack(false)
				} else {
					// 其他类型错误，才要求重试
					logCtx.WithError(opErr).Error("处理消息失败，将进行重试")
					d.Nack(false, true)
				}
			} else {
				// 通知mq处理失败，并将消息删除
				d.Ack(false)
			}
		}
	}()
	logger.Log.Info(" [*] 等待点赞消息中. 按 CTRL+C 退出")
	// 尝试从forever通道里接收一个值，但没有发送者，这会阻止main函数退出
	<-forever
}

// 黄金评论消费者：1、通过amqp.Connection建立channel，并设置channel为消费者 2、建立轮询，读取channel 3、利用消息结构体反序列化消息，并用事务单元保证“一荣俱荣，一损俱损” 4、利用videoID找到视频，并使用ForUpadate加锁，锁住video对象，判断时间（<10min）和数量()
func consumeGoldenComments(conn *amqp.Connection, db *gorm.DB, commentRepo repository.CommentRepository, uow data.UnitOfWork) {

	ch, err := conn.Channel()
	if err != nil {
		logger.Log.Fatalf("无法打开Channel: %v", err)
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		QueueGoldenComment, // queue
		"",                 // consumer
		false,              // auto-ack: 为简单起见，我们先用自动确认
		false,              // exclusive
		false,              // no-local
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		logger.Log.Fatalf("无法注册黄金评论消费者: %v", err)
	}
	// 创建一个没有任何缓冲的bool类型通道
	forever := make(chan bool)

	go func() {
		// msgs不是切片，而是通道channel，如果通道为空不会结束循环，而会“阻塞”
		for d := range msgs {
			logCtx := logger.Log.WithField("message_id", d.MessageId).WithField("redelivered", d.Redelivered)
			logCtx.Info("收到一条黄金评论！")
			var msgGolden GoldenCommentMessage
			if err := json.Unmarshal([]byte(d.Body), &msgGolden); err != nil {
				logCtx.WithError(err).Error("消息JSON解析失败")
				// 永久性错误，Ack掉
				d.Ack(false)
				continue
			}
			logCtx = logCtx.WithField("user_id", msgGolden.UserID).WithField("video_id", msgGolden.VideoID)
			// 使用“工作单元”来执行我们的事务性操作
			err := uow.Execute(func(repos *data.TransactionalRepositories) error {

				newComment := &model.Comment{
					UserID:   msgGolden.UserID,
					VideoID:  msgGolden.VideoID,
					Content:  msgGolden.Content,
					IsGolden: true,
				}
				// Create会被GORM翻译成INSERT语句，数据库在执行INSERT时是原子的，并且会对新插入的行加锁，所以无需手动加锁
				if err := repos.CommentRepo.Create(newComment); err != nil {
					// 这里的错误可能是“重复键”，也可能是数据库连接问题
					return err
				}
				// FindByIDForUpdate时，就获取了这行数据的锁，所以无需额外加锁
				if _, err := repos.VideoRepo.IncrementGoldenCount(newComment.VideoID); err != nil {
					return err
				}
				// 函数正常返回nil，UoW会帮我们提交事务，否则回滚整个事务
				return nil
			})
			// 根据数据库操作的结果，来决定如何“确认”消息
			if err != nil {
				var mysqlErr *mysql.MySQLError
				// 判断是不是可被认为是“成功”的幂等性错误
				if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
					// 错误号 1062 就是 "Duplicate entry"
					logCtx.WithError(err).Warn("处理消息时出现重复键错误，可能是一次重复消费，消息将被确认为成功。")
					// 这不是一个需要重试的错误，直接Ack掉
					d.Ack(false)
				} else {
					// 其他类型错误，才要求重试
					logCtx.WithError(err).Error("处理消息失败，将进行重试")
					d.Nack(false, true)
				}
			} else {
				// 通知mq处理失败，并将消息删除
				d.Ack(false)
			}
		}
	}()
	logger.Log.Info(" [*] 等待“黄金评论”消息中. 按 CTRL+C 退出")
	// 尝试从forever通道里接收一个值，但没有发送者，这会阻止main函数退出
	<-forever
}

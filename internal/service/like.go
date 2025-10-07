package service

import (
	"Orion_Live/internal/repository"
	"encoding/json"
	"errors"

	"github.com/streadway/amqp"
	"gorm.io/gorm"
)

const (
	// 遵循：项目名.业务领域.实体/功能
	QueueLike    = "orion.like.queue" // 定义队列名称
	ActionLike   = "like"
	ActionUnlike = "unlike"
)

// LikeMessage 定义了我们要在MQ中传递的消息结构
type LikeMessage struct {
	UserID  uint64 `json:"user_id"`
	VideoID uint64 `json:"video_id"`
	Action  string `json:"action"` // "like" or "unlike"
}

// redis + mq
type LikeService interface {
	LikeVideo(userID, videoID uint64) error
	UnlikeVideo(userID, videoID uint64) error
}

// videoRepo用于redis查重+redis插入，rabbitMQConn用于持久化（mysql）
type likeService struct {
	videoRepo    repository.VideoRepository
	rabbitMQConn *amqp.Connection
}

func NewLikeService(videoRepo repository.VideoRepository, rabbitMQConn *amqp.Connection) LikeService {
	ch, err := rabbitMQConn.Channel()
	if err != nil {
		// 在实际项目中，这里应该有更健壮的错误处理和重试机制
		panic("Failed to open a channel")
	}
	// NewLikeService执行完毕后，这个临时的Channel就被关闭了
	defer ch.Close()
	// 创建名叫“orion.like.queue”的邮筒，有就不用创建（幂等）
	_, err = ch.QueueDeclare(
		QueueLike, // name
		true,      // durable: 队列持久化，即使RabbitMQ服务器重启，这个“邮筒”本身不会消失（注：里面的信件是否消失，取决于信件本身的持久化设置）
		false,     // autoDelete：最后一个消费者断开连接，邮筒不会被自动拆除
		false,     // exclusive：非独占，多个不同的连接都可以访问这个邮筒
		false,     // noWait：同步等待，QueueDeclare会等待RabbitMQ服务器确认“邮筒建好了”之后，再继续执行后面的代码
		nil,       // args
	)
	if err != nil {
		panic("Failed to declare a queue")
	}

	return &likeService{
		videoRepo:    videoRepo,
		rabbitMQConn: rabbitMQConn,
	}
}

// 点赞视频：1、检查点赞的视频是否存在 2、检查用户是否已点赞 3、redis点赞视频 4、通过LikeMessage发布“点赞视频”消息
// 这里其实有问题，因为FindByID检查的是数据库，我们第一时间操作的是redis，如果没有限制还好，有限制就不对
func (s *likeService) LikeVideo(userID, videoID uint64) error {
	_, err := s.videoRepo.FindByID(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("视频不存在")
		}
		return err
	}
	liked, err := s.videoRepo.IsUserLikeVideo(videoID, userID)
	if err != nil {
		// Redis出错
		return err
	}
	if liked {
		return errors.New("您已经点赞过该视频")
	}

	if err := s.videoRepo.AddVideoLike(videoID, userID); err != nil {
		return err
	}

	// 发送异步消息，通知后台去写数据库
	msg := LikeMessage{UserID: userID, VideoID: videoID, Action: ActionLike}
	return s.publishLikeMessage(msg)
}

// 取消点赞：1、检查取赞的视频是否存在 2、检查用户是否已点赞 3、redis取消点赞视频 4、通过LikeMessage发布“取消点赞视频”消息
func (s *likeService) UnlikeVideo(userID, videoID uint64) error {
	_, err := s.videoRepo.FindByID(videoID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("视频不存在")
		}
		return err
	}
	liked, err := s.videoRepo.IsUserLikeVideo(videoID, userID)
	if err != nil {
		return err
	}
	if !liked {
		return errors.New("您还未点赞该视频")
	}
	// 执行取消点赞操作 (Redis)
	if err := s.videoRepo.RemoveVideoLike(videoID, userID); err != nil {
		return err
	}

	// 发送异步消息
	msg := LikeMessage{UserID: userID, VideoID: videoID, Action: ActionUnlike}
	return s.publishLikeMessage(msg)
}

// 私有方法，发送消息到RabbitMQ：1、创建channel 2、序列化LikeMessage结构体 3、发布消息
func (s *likeService) publishLikeMessage(msg LikeMessage) error {
	// 为每一个消息建立一个单独的channel，消息之间互不影响
	// publishLikeMessage执行完毕后，这个临时的Channel就被关闭了
	ch, err := s.rabbitMQConn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	// 将msg结构体序列化成一段JSON格式的字节流([]byte)
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return ch.Publish(
		"",        // exchange默认交换机
		QueueLike, // routing key “邮筒”名字 orion.like.queue
		false,     // mandatory 高级路由功能
		false,     // immediate 高级路由功能
		// 信件本身
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body, //序列化的msg结构体
		})
}

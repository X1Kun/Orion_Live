package service

import (
	"Orion_Live/internal/data"
	"Orion_Live/internal/model"
	"Orion_Live/internal/repository"
	"Orion_Live/pkg/logger"
	"encoding/json"
	"errors"

	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
)

const (
	QueueGoldenComment = "orion.golden_comment.queue"
)

// GoldenCommentMessage 定义了黄金评论的消息结构
type GoldenCommentMessage struct {
	UserID  uint64 `json:"user_id"`
	VideoID uint64 `json:"video_id"`
	Content string `json:"content"`
}

type CommentService interface {
	// 创建视频的一级评论
	CreateComment(userID, videoID uint64, content string) (*model.Comment, error)
	// 创建视频的一级评论
	CreateReply(userID uint64, parentComment *model.Comment, content string) (*model.Comment, error)

	CreateGoldenComment(userID, videoID uint64, content string) (*model.Comment, error)
	// 获取一个视频的所有评论
	GetComments(videoID uint64, page, pageSize int) ([]model.Comment, map[uint64][]*model.Comment, error)
}

type commentService struct {
	commentRepo repository.CommentRepository
	videoRepo   repository.VideoRepository
	uow         data.UnitOfWork

	rdb          *redis.Client
	rabbitMQConn *amqp.Connection
}

type CommentsWithReplies struct {
	ParentComments []model.Comment
	ReplyMap       map[uint64][]*model.Comment
}

func NewCommentService(commentRepo repository.CommentRepository, videoRepo repository.VideoRepository, uow data.UnitOfWork, rdb *redis.Client, conn *amqp.Connection) CommentService {

	ch, _ := conn.Channel()
	defer ch.Close()
	ch.QueueDeclare(QueueGoldenComment, true, false, false, false, nil)

	return &commentService{
		commentRepo:  commentRepo,
		videoRepo:    videoRepo,
		uow:          uow,
		rdb:          rdb,
		rabbitMQConn: conn,
	}
}

// 创建一级评论：1、创建一级评论 2、利用一级评论的ID查找，Preload出User以及空的ReplyToUser
func (s *commentService) CreateComment(userID, videoID uint64, content string) (*model.Comment, error) {
	newComment := &model.Comment{
		UserID:    userID,
		VideoID:   videoID,
		Content:   content,
		LikeCount: 0,
		IsGolden:  false,
		// ParentID 和 ReplyToUserID 都是零值(nil)
	}
	if err := s.commentRepo.Create(newComment); err != nil {
		return nil, err
	}
	// 创建成功后，立刻把它带着关联数据再查出来，FindByID就能顺带Preload出newComment的User和ReplyToUser结构体
	return s.commentRepo.FindByID(newComment.ID)
}

// 创建二级评论：1、利用上层传来的UserID和content，以及commentID找到的一级评论，构建并创建二级评论 2、利用返回的二级评论ID进行查找，Preload出二级评论的User和ReplyToUser
func (s *commentService) CreateReply(userID uint64, parentComment *model.Comment, content string) (*model.Comment, error) {
	if parentComment.ParentID != nil {
		return nil, errors.New("不能对二级评论进行回复")
	}
	newReply := &model.Comment{
		UserID:        userID,
		VideoID:       parentComment.VideoID,
		Content:       content,
		LikeCount:     0,
		IsGolden:      false,
		ParentID:      &parentComment.ID,
		ReplyToUserID: &parentComment.UserID,
	}
	if err := s.commentRepo.Create(newReply); err != nil {
		return nil, err
	}
	// 创建成功后，通过Preload获取数据的完整Comment对象
	return s.commentRepo.FindByID(newReply.ID)
}

// 创建黄金评论：1、先利用videoID在redis中抢占席位，返回现在的黄金评论数 2、判断席位数，如果超限，则返回归还席位 3、满足则构建消息，并发送消息至rabbitMQ
func (s *commentService) CreateGoldenComment(userID, videoID uint64, content string) (*model.Comment, error) {

	count, err := s.videoRepo.IncrementGoldenCount_Redis(videoID)
	if err != nil {
		return nil, errors.New("系统繁忙，请稍后再试 (Redis错误)")
	}
	// 判断抢占结果
	if count > 100 {
		// 席位已满，需要执行“补偿”操作，把刚刚多加的那个数减回去
		_ = s.videoRepo.DecrementGoldenCount_Redis(videoID)
		return nil, errors.New("黄金评论席已满")
	}
	// 发送异步消息到 RabbitMQ
	msg := GoldenCommentMessage{
		UserID:  userID,
		VideoID: videoID,
		Content: content,
	}
	if err := s.publishGoldenCommentMessage(msg); err != nil {
		// 这是最严重的错误：Redis 已经扣了库存，但消息没发出去
		// 必须补偿Redis，并记录严重错误日志以供人工排查
		_ = s.videoRepo.DecrementGoldenCount_Redis(videoID)
		logger.Log.WithError(err).
			WithField("user_id", userID).
			WithField("video_id", videoID).
			Error("【严重】黄金评论消息投递失败！Redis库存已扣减，需人工核对！")
		return nil, errors.New("系统错误，评论失败")
	}

	// 发送成功！返回一个临时Comment对象给前端，用于“乐观UI”
	tempComment := &model.Comment{
		UserID:   userID,
		VideoID:  videoID,
		Content:  content,
		IsGolden: true,
	}

	return tempComment, nil
}

// (私有方法) publishGoldenCommentMessage - 类似 LikeService 的实现
func (s *commentService) publishGoldenCommentMessage(msg GoldenCommentMessage) error {
	ch, err := s.rabbitMQConn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return ch.Publish(
		"",                 // exchange
		QueueGoldenComment, // routing key
		false,              // mandatory
		false,              // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // 确保消息持久化
		})
}

// 获取视频的评论列表：1、计算分页参数 2、根据videoID查询一级评论 3、根据一级评论的ID切片查询所有二级评论 4、将二级评论挂载（map）到一级评论下并返回CommentsWithReplies结构体
func (s *commentService) GetComments(videoID uint64, page, pageSize int) ([]model.Comment, map[uint64][]*model.Comment, error) {
	// pageSize：每页大小。page:当前页码。offset: “跳过” 多少条记录，再开始取数据。
	offset := (page - 1) * pageSize
	// 查询一级评论
	parentComments, err := s.commentRepo.GetCommentsByVideoID(videoID, offset, pageSize)
	if err != nil {
		return nil, nil, err
	}
	if len(parentComments) == 0 {
		return nil, nil, nil // 如果没有一级评论，直接返回空列表
	}
	// 创建切片，将每个一级评论的ID放入，方便二级评论查询
	parentIDs := make([]uint64, 0, len(parentComments))
	for _, pc := range parentComments {
		parentIDs = append(parentIDs, pc.ID)
	}
	// 一次性查询所有相关的二级评论
	replies, err := s.commentRepo.GetRepliesByParentIDs(parentIDs)
	if err != nil {
		return nil, nil, err
	}
	// 在内存中进行数据编排，将二级评论挂载到对应的一级评论上
	replyMap := make(map[uint64][]*model.Comment)
	for i := range replies {
		reply := replies[i]
		if reply.ParentID != nil {
			replyMap[*reply.ParentID] = append(replyMap[*reply.ParentID], &reply)
		}
	}
	// 将打包结果返回
	return parentComments, replyMap, nil
}

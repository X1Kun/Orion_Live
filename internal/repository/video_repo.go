package repository

import (
	"Orion_Live/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	keyVideoLikeCountHash   = "video:like_counts"
	keyVideoGoldenCountHash = "video:golden_counts"
	keyVideoLikersSet       = "video:likers"
)

type VideoRepository interface {
	Create(video *model.Video) error
	FindLatest(limit uint64) ([]model.Video, error)
	FindByID(videoID uint64) (*model.Video, error)
	// 带锁的查找
	FindByIDForUpdate(videoID uint64) (*model.Video, error)
	IncrementLikeCount(videoID uint64) error
	DecrementLikeCount(videoID uint64) error

	GetGoldenCount(videoID uint64) (uint64, error)
	IncrementGoldenCount(videoID uint64) (uint64, error)
	IncrementGoldenCount_Redis(videoID uint64) (uint64, error) // 返回增长后的计数值
	DecrementGoldenCount_Redis(videoID uint64) error           // 用于补偿

	GetVideoCache(videoID uint64) (*model.Video, error)
	SetVideoCache(video *model.Video) error

	// Redis的所有值（Value）都是二进制安全的字符串
	AddVideoLike(videoID, userID uint64) error
	RemoveVideoLike(videoID, userID uint64) error
	GetVideoLikeCount(videoID uint64) (uint64, error)
	IsUserLikeVideo(videoID, userID uint64) (bool, error)

	WithTx(tx *gorm.DB) VideoRepository
}

type videoRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewVideoRepository(db *gorm.DB, rdb *redis.Client) VideoRepository {
	return &videoRepository{
		db:  db,
		rdb: rdb,
	}
}

// WithTx 返回一个新的、使用事务的 commentRepository 实例
func (r *videoRepository) WithTx(tx *gorm.DB) VideoRepository {
	return &videoRepository{
		db: tx,
	}
}

func (r *videoRepository) Create(video *model.Video) error {
	return r.db.Create(video).Error
}

// 按时间倒序查询最新的视频列表
func (r *videoRepository) FindLatest(limit uint64) ([]model.Video, error) {
	var videos []model.Video

	// Preload("Author")在查询视频的同时，预加载关联的作者信息,时间倒序,限制数量
	err := r.db.Preload("Author").Order("created_at desc").Limit(int(limit)).Find(&videos).Error
	if err != nil {
		return nil, err
	}
	return videos, nil
}

// 利用videoID找视频，preload其中的Author结构
func (r *videoRepository) FindByID(videoID uint64) (*model.Video, error) {
	// 1. 先从缓存读
	video, err := r.GetVideoCache(videoID)
	if err == nil && video != nil {
		// 缓存命中，直接返回
		return video, nil
	}

	// 2. 缓存未命中，从数据库读
	var dbVideo model.Video
	err = r.db.Preload("Author").First(&dbVideo, videoID).Error
	if err != nil {
		return nil, err // 数据库也没找到，就真的没有了
	}

	// 3. 读到数据后，写回缓存，方便下次读取
	_ = r.SetVideoCache(&dbVideo)

	return &dbVideo, nil
}

func (r *videoRepository) FindByIDForUpdate(videoID uint64) (*model.Video, error) {
	var video model.Video
	// SELECT * FROM `videos` WHERE `id` = ? LIMIT 1 FOR UPDATE;
	// Clauses允许在生成的SQL语句后面追加子句，比如排序、分组，以及加锁
	// clause.Locking，是GORM预定义好的锁条款的结构体。Strength: "UPDATE"，锁的强度，排他锁（Exclusive Lock）
	// FOR UPDATE锁的生命周期和事务的生命周期是完全绑定的，会持续直到整个Execute函数包裹的事务结束
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&video, videoID).Error
	return &video, err
}

// 返回存储单个视频信息的字符串Key
func (r *videoRepository) keyVideoInfo(videoID uint64) string {
	return fmt.Sprintf("video:info:%d", videoID)
}

// 从Redis缓存中获取单个Video信息：1、利用VideoID组装key 2、拿key去rdb中寻找videoJSON 3、利用json.Unmarshal将拿到的videoJSON反序列化
func (r *videoRepository) GetVideoCache(videoID uint64) (*model.Video, error) {
	// 将videoID拼装成video:info:{videoID}格式
	key := r.keyVideoInfo(videoID)
	// 使用GET命令获取存储在rdb里的JSON字符串
	videoJSON, err := r.rdb.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return nil, nil // 如果缓存不存在，但是Redis正常工作
	} else if err != nil {
		return nil, err // Redis本身出错了
	}
	// 将获取到的JSON字符串，反序列化回model.Video结构体
	var video model.Video
	if err := json.Unmarshal([]byte(videoJSON), &video); err != nil {
		return nil, err // JSON反序列化失败
	}
	return &video, nil
}

// 将单个视频信息存入Redis缓存：1、利用传入的video的videoID拼装成video:info:{videoID}格式，形成key 2、将video结构体，序列化成JSON字符串 3、设置过期时间 4、使用Set命令将序列化的VideoJSON存入Redis
func (r *videoRepository) SetVideoCache(video *model.Video) error {
	key := r.keyVideoInfo(video.ID)
	// 将model.Video结构体，序列化成JSON字符串
	videoJSON, err := json.Marshal(video)
	if err != nil {
		return err // JSON序列化失败
	}
	// 设置过期时间，再加上随机性防止缓存雪崩
	expiration := time.Minute*5 + time.Duration(rand.Intn(60))*time.Second
	// 使用SET命令将JSON字符串存入Redis，并设置过期时间
	return r.rdb.Set(context.Background(), key, videoJSON, expiration).Err()
}

func (r *videoRepository) keyVideoGoldenCount(videoID uint64) string {
	return fmt.Sprintf("video:golden_count:%d", videoID)
}

func (r *videoRepository) IncrementLikeCount(videoID uint64) error {
	// 使用GORM的表达式来执行原子更新：UPDATE `videos` SET `like_count` = `like_count` + 1 WHERE id = ?
	return r.db.Model(&model.Video{}).Where("id = ?", videoID).UpdateColumn("like_count", gorm.Expr("like_count + ?", 1)).Error
}

func (r *videoRepository) DecrementLikeCount(videoID uint64) error {
	// UPDATE `videos` SET `like_count` = `like_count` - 1 WHERE id = ? AND like_count > 0
	return r.db.Model(&model.Video{}).Where("id = ? AND like_count > 0", videoID).UpdateColumn("like_count", gorm.Expr("like_count - ?", 1)).Error
}

// 在Redis中增加黄金评论数量
func (r *videoRepository) IncrementGoldenCount_Redis(videoID uint64) (uint64, error) {
	key := r.keyVideoGoldenCount(videoID)
	// INCR本身是原子的
	res, err := r.rdb.Incr(context.Background(), key).Result()
	return uint64(res), err
}

func (r *videoRepository) DecrementGoldenCount_Redis(videoID uint64) error {
	key := r.keyVideoGoldenCount(videoID)
	return r.rdb.Decr(context.Background(), key).Err()
}

func (r *videoRepository) IncrementGoldenCount(videoID uint64) (uint64, error) {
	err := r.db.Model(&model.Video{}).Where("id = ?", videoID).UpdateColumn("golden_count", gorm.Expr("golden_count + ?", 1)).Error
	return 0, err
}

func (r *videoRepository) GetGoldenCount(videoID uint64) (uint64, error) {
	videoIDStr := strconv.FormatUint(videoID, 10)
	// 虽然redis是“键值数据库”，但是储存后拿出来，都是字符串string，所以之后要转化
	countStr, err := r.rdb.HGet(context.Background(), keyVideoGoldenCountHash, videoIDStr).Result()
	if err == redis.Nil {
		return 0, nil // 如果key或field不存在，返回0
	} else if err != nil {
		return 0, err
	}
	count, _ := strconv.ParseUint(countStr, 10, 64)
	return count, nil
}

// 增加点赞记录：1、将用户ID添加到视频点赞集合 2、视频点赞数++ 3、利用r.rdb.Pipeline()保证操作的原子性
func (r *videoRepository) AddVideoLike(videoID, userID uint64) error {
	videoIDStr := strconv.FormatUint(videoID, 10)
	userIDStr := strconv.FormatUint(userID, 10)
	// Pipeline保证多个命令的原子性执行，并减少网络往返，打包发送
	pipe := r.rdb.Pipeline()
	// videoID找set，userID添加到set中
	pipe.SAdd(context.Background(), keyVideoLikersSet+":"+videoIDStr, userIDStr)
	// 利用哈希表给表中的“Field”为videoID的，加了个1
	pipe.HIncrBy(context.Background(), keyVideoLikeCountHash, videoIDStr, 1)

	_, err := pipe.Exec(context.Background())
	return err
}

// 移除点赞记录：1、将用户ID从视频点赞集合中删除 2、视频点赞数-- 3、利用r.rdb.Pipeline()保证操作的原子性
func (r *videoRepository) RemoveVideoLike(videoID, userID uint64) error {
	videoIDStr := strconv.FormatUint(videoID, 10)
	userIDStr := strconv.FormatUint(userID, 10)
	pipe := r.rdb.Pipeline()
	pipe.SRem(context.Background(), keyVideoLikersSet+":"+videoIDStr, userIDStr)
	pipe.HIncrBy(context.Background(), keyVideoLikeCountHash, videoIDStr, -1)
	_, err := pipe.Exec(context.Background())
	return err
}

// 获取视频的点赞总数 1、利用videoID拼接成video:like_counts这个哈希结构 2、利用videoID这个field获取like_counts值
func (r *videoRepository) GetVideoLikeCount(videoID uint64) (uint64, error) {
	videoIDStr := strconv.FormatUint(videoID, 10)
	// 虽然redis是“键值数据库”，但是储存后拿出来，都是字符串string，所以之后要转化
	countStr, err := r.rdb.HGet(context.Background(), keyVideoLikeCountHash, videoIDStr).Result()
	if err == redis.Nil {
		return 0, nil // 如果key或field不存在，返回0
	} else if err != nil {
		return 0, err
	}
	count, _ := strconv.ParseUint(countStr, 10, 64)
	return count, nil
}

// 判断用户是否点赞过该视频：1、利用传来的videoID和userID，在video:likers:{videoID}这个set中找是否有userID
func (r *videoRepository) IsUserLikeVideo(videoID, userID uint64) (bool, error) {
	videoIDStr := strconv.FormatUint(videoID, 10)
	userIDStr := strconv.FormatUint(userID, 10)
	return r.rdb.SIsMember(context.Background(), keyVideoLikersSet+":"+videoIDStr, userIDStr).Result()
}

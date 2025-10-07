package service

import (
	"Orion_Live/internal/model"
	"Orion_Live/internal/repository"
	"fmt"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
)

type VideoService interface {
	CreateVideo(authorID uint64, title, description string) (*model.Video, error)
	GetFeed(limit uint64) ([]model.Video, error)

	GetVideoByID(videoID uint64) (*model.Video, error)
}

type videoService struct {
	sf singleflight.Group

	videoRepo repository.VideoRepository
}

func NewVideoService(videoRepo repository.VideoRepository) VideoService {
	return &videoService{
		videoRepo: videoRepo,
	}
}

func (s *videoService) CreateVideo(authorID uint64, title, description string) (*model.Video, error) {
	newVideo := &model.Video{
		AuthorID:    uint64(authorID),
		Title:       title,
		Description: description,
		VideoURL:    "https://placeholder.com/video.mp4",
		CoverURL:    "https://placeholder.com/cover.jpg",
	}
	err := s.videoRepo.Create(newVideo)
	if err != nil {
		return nil, err
	}
	return newVideo, nil
}

// 获取视频Feed流
func (s *videoService) GetFeed(limit uint64) ([]model.Video, error) {
	// 限制limit长度
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	videos, err := s.videoRepo.FindLatest(limit)
	if err != nil {
		return nil, err
	}

	return videos, nil
}

// 根据videoID查找视频：1、查找Redis缓存 2、通过SingleFlight进行数据库查找
func (s *videoService) GetVideoByID(videoID uint64) (*model.Video, error) {

	video, err := s.videoRepo.GetVideoCache(videoID)
	if err == nil && video != nil {
		fmt.Println("我从缓存里拿到数据了！")
		return video, nil
	}
	// 不是redis中没有，而是Redis本身出错了，应该记录日志并返回
	if err != nil && err != redis.Nil {
		return nil, err
	}
	// 缓存未命中，通过SingleFlight查找，同一时间执行的
	key := fmt.Sprintf("get_video_%d", videoID)
	result, err, _ := s.sf.Do(key, func() (interface{}, error) {
		dbVideo, dbErr := s.videoRepo.FindByID(videoID)
		if dbErr != nil {
			return nil, dbErr
		}
		// 查询成功后，将返回的dbVideo写回缓存！
		_ = s.videoRepo.SetVideoCache(dbVideo)
		return dbVideo, nil
	})
	if err != nil {
		return nil, err
	}
	// 虽然找到了videoID对应的视频，但返回值是interface{}结构，需要断言
	return result.(*model.Video), nil
}

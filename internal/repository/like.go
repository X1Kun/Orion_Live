package repository

import (
	"Orion_Live/internal/model"
	"Orion_Live/pkg/logger"

	"gorm.io/gorm"
)

type LikeRepository interface {
	Create(like *model.Like) error
	Delete(userID, videoID uint64) error
}

type likeRepository struct {
	db *gorm.DB
}

func NewLikeRepository(db *gorm.DB) LikeRepository {
	return &likeRepository{db: db}
}

func (r *likeRepository) Create(like *model.Like) error {

	// logger.Log.Infof("准备从MySQL添加点赞记录: UserID=%d, VideoID=%d", like.UserID, like.VideoID)
	result := r.db.Create(like)
	if result.Error != nil {
		logger.Log.WithError(result.Error).Error("MySQL添加操作失败")
		return result.Error
	}

	// logger.Log.Infof("MySQL添加操作完成，影响行数: %d", result.RowsAffected)

	return nil
}

func (r *likeRepository) Delete(userID, videoID uint64) error {

	// logger.Log.Infof("准备从MySQL删除点赞记录: UserID=%d, VideoID=%d", userID, videoID)
	// gorm简直就是dogShit，排查了将近两个小时的错误，结果就真是gorm的“翻译”错误
	// ↓ 这句是错的
	// result := r.db.Where("user_id = ? AND video_id = ?", userID, videoID).Delete(&model.Like{})
	result := r.db.Exec("DELETE FROM likes WHERE user_id = ? AND video_id = ?", userID, videoID)
	if result.Error != nil {
		logger.Log.WithError(result.Error).Error("MySQL删除操作失败")
		return result.Error
	}

	// logger.Log.Infof("MySQL删除操作完成，影响行数: %d", result.RowsAffected)

	return nil
}

package repository

import (
	"Orion_Live/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommentRepository interface {
	Create(comment *model.Comment) error
	FindByID(commentID uint64) (*model.Comment, error)
	CreateInTx(tx *gorm.DB, comment *model.Comment) error

	// 分页获取视频的一级评论
	GetCommentsByVideoID(videoID uint64, offset, limit int) ([]model.Comment, error)
	// 根据父评论ID列表，获取二级评论
	GetRepliesByParentIDs(parentIDs []uint64) ([]model.Comment, error)

	WithTx(tx *gorm.DB) CommentRepository
}

type commentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &commentRepository{db: db}
}

// WithTx 返回一个新的、使用事务的 commentRepository 实例
func (r *commentRepository) WithTx(tx *gorm.DB) CommentRepository {
	return &commentRepository{
		db: tx,
	}
}

// Create 方法现在对事务和非事务场景通用
func (r *commentRepository) Create(comment *model.Comment) error {
	return r.db.Create(comment).Error
}

func (r *commentRepository) CreateInTx(tx *gorm.DB, comment *model.Comment) error {
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Create(comment).Error
}

// 利用commentID找comment，并顺便将结构体中的User和ReplyToUser给Preload进去
func (r *commentRepository) FindByID(commentID uint64) (*model.Comment, error) {

	var result model.Comment
	// err := r.db.Where("id = ?", commentID).First(&result).Error
	// 更简洁的方式，将筛选条件放在db.First参数中
	// 并且把Comment结构体中的User和ReplyToUser结构体也Preload出来
	err := r.db.Preload("User").Preload("ReplyToUser").First(&result, commentID).Error
	if err != nil {
		return nil, err // 如果有错（包括没找到），直接返回想
	}
	return &result, err
}

// 分页获取一个视频下的一级评论
func (r *commentRepository) GetCommentsByVideoID(videoID uint64, offset, limit int) ([]model.Comment, error) {
	var comments []model.Comment
	err := r.db.
		Preload("User"). // 预加载评论的作者信息，能一次性地把作者、被回复者等所有关联信息查询出来
		Where("video_id = ? AND parent_id IS NULL", videoID).
		Offset(offset).
		Limit(limit).
		Order("created_at desc").
		Find(&comments).Error
	return comments, err
}

// 根据一批父评论ID，获取它们所有的二级评论
func (r *commentRepository) GetRepliesByParentIDs(parentIDs []uint64) ([]model.Comment, error) {
	var replies []model.Comment
	err := r.db.
		Preload("User").        // 预加载二级评论的作者
		Preload("ReplyToUser"). // 预加载被回复者的信息
		Where("parent_id IN (?)", parentIDs).
		Order("created_at asc"). // 二级评论通常按时间正序排列
		Find(&replies).Error
	return replies, err
}

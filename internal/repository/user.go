package repository

import (
	"Orion_Live/internal/model"

	"gorm.io/gorm"
)

// 用户仓库接口：1、将用户插入用户表 2、根据用户名查找用户 3、根据用户名和密码验证用户
type UserRepository interface {
	Create(user *model.User) error
	FindByUsername(username string) (*model.User, error)
}

// 数据库接口封装
type userRepository struct {
	db *gorm.DB
}

// 封装函数
func NewUserRepository(db *gorm.DB) *userRepository {
	return &userRepository{db: db}
}

// 用户插入表
func (r *userRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

// 根据用户名找用户
func (r *userRepository) FindByUsername(username string) (*model.User, error) {
	var result model.User
	err := r.db.Where("username = ?", username).First(&result).Error
	if err != nil {
		return nil, err // 如果有错（包括没找到），直接返回
	}
	return &result, err
}

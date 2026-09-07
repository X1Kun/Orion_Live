package repository

import (
	"context"

	"github.com/X1Kun/orion-live/internal/model"

	"gorm.io/gorm"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a GORM-backed user repository.
func NewUserRepository(db *gorm.DB) *userRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var result model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

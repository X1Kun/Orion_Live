package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/X1Kun/orion-live/internal/model"
	"github.com/X1Kun/orion-live/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrInvalidUsername    = errors.New("username must contain 3 to 64 characters")
	ErrInvalidPassword    = errors.New("password must contain 8 to 72 characters")
)

type UserService interface {
	Register(ctx context.Context, username, password string) (*model.User, error)
	Login(ctx context.Context, username, password string) (string, error)
}

type userService struct {
	users       repository.UserRepository
	jwtSecret   []byte
	accessToken time.Duration
}

func NewUserService(users repository.UserRepository, jwtSecret string, accessTokenTTL time.Duration) UserService {
	return &userService{users: users, jwtSecret: []byte(jwtSecret), accessToken: accessTokenTTL}
}

func (s *userService) Register(ctx context.Context, username, password string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 {
		return nil, ErrInvalidUsername
	}
	if len(password) < 8 || len(password) > 72 {
		return nil, ErrInvalidPassword
	}

	if _, err := s.users.FindByUsername(ctx, username); err == nil {
		return nil, ErrUsernameTaken
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{Username: username, PasswordHash: string(hash)}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}
	return user, nil
}

func (s *userService) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(user.ID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.accessToken)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

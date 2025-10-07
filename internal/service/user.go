package service

import (
	"Orion_Live/internal/model"
	"Orion_Live/internal/repository"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 用户服务接口：1、注册 2、登录
type UserService interface {
	Register(username, password string) (*model.User, error)
	Login(username, password string) (string, error)
}

// 用户服务包装
type userService struct {
	userRepo repository.UserRepository
}

// 包装函数
func NewUserService(userRepo repository.UserRepository) *userService {
	return &userService{userRepo: userRepo}
}

// 注册逻辑：1、检查是否重名 2、密码加密存储 3、创建用户表项 4、插入数据库
func (s *userService) Register(username, password string) (*model.User, error) {
	_, err := s.userRepo.FindByUsername(username)
	if err == nil {
		return nil, errors.New("用户名已存在")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	newUser := &model.User{
		Username: username,
		Password: string(hashedPassword),
	}

	err = s.userRepo.Create(newUser)
	if err != nil {
		return nil, err
	}
	return newUser, nil
}

// 登录逻辑：1、检查库中是否有该用户名 2、加密后密码和输入密码比对 3、生成jwt签名
func (s *userService) Login(username, password string) (string, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("用户名不存在")
		}
		return "", err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", errors.New("用户名或密码错误")
	}
	// token对象的Payload，不能将密码放在其中，Payload不加密
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 72).Unix(), // 过期时间，这里设置为72小时
		"iat":      time.Now().Unix(),                     // 签发时间
	}
	// token加上Header，算法信息HS256，对称加密
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secretKey := []byte(os.Getenv("JWT_SECRET_KEY"))
	// 对token对象中的Header和Payload进行签名，用于防伪（Header.Payload.Signature）
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		// 生成token失效
		return "", err
	}

	return tokenString, nil
}

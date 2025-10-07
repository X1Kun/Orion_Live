package handler

import (
	"Orion_Live/internal/service"
	"Orion_Live/pkg/logger"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	GetProfile(c *gin.Context)
}

// 对Service进行封装
type userHandler struct {
	UserService service.UserService
}

// 封装函数
func NewUserHandler(userService service.UserService) UserHandler {
	return &userHandler{UserService: userService}
}

// 用处：接收http发来的全部注册信息，用户名+密码
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// 注册：1、URL解析为注册请求结构体 2、service层利用Username和Password进行注册 3、返回注册成功后的User
func (h *userHandler) Register(c *gin.Context) {

	var req RegisterRequest
	// c.ShouldBindJSON，绑定和校验，如果context中不包含req的“required”字段，则会返回错误
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.WithError(err).Error("请求参数解析失败")
		sendErrorResponse(c, http.StatusBadRequest, "无效的参数")
		return
	}

	logCtx := logger.Log.WithField("username", req.Username)
	logCtx.Info("开始处理用户注册请求")

	user, err := h.UserService.Register(req.Username, req.Password)
	if err != nil {
		logCtx.WithError(err).Error("用户注册业务逻辑处理失败")
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	logCtx.WithField("user_id", user.ID).Info("用户注册成功")

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

// 登录：1、URL解析为登录结构体 2、Username和Password传给service层，登录服务 3、成功则返回token
func (h *userHandler) Login(c *gin.Context) {

	var login LoginRequest
	// c.ShouldBindJSON，绑定和校验，如果context中不包含req的“required”字段，则会返回错误
	if err := c.ShouldBindJSON(&login); err != nil {
		logger.Log.WithError(err).Error("登录请求参数解析失败")
		sendErrorResponse(c, http.StatusBadRequest, "无效的参数")
		return
	}

	logCtx := logger.Log.WithField("username", login.Username)
	logCtx.Info("开始处理用户登录请求")

	token, err := h.UserService.Login(login.Username, login.Password)
	if err != nil {
		logCtx.WithError(err).Error("用户登录业务逻辑处理失败")
		// 模糊的错误提示，更安全
		sendErrorResponse(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	logCtx.Info("用户登录成功")

	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"data": gin.H{
			"token": token,
		},
	})
}

// 获取用户个人信息：1、从context获取认证后用户的userID和Username
func (h *userHandler) GetProfile(c *gin.Context) {
	// 从以及认证后的Context中获取用户信息
	userID, exists := c.Get("userID")
	if !exists {
		sendErrorResponse(c, http.StatusUnauthorized, "用户未认证")
		return
	}
	username, _ := c.Get("username")

	c.JSON(http.StatusOK, gin.H{
		"message": "成功获取用户信息",
		"data": gin.H{
			"user_id":  userID,
			"username": username,
		},
	})
}

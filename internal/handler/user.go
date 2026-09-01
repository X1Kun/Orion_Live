package handler

import (
	"errors"
	"net/http"

	"github.com/X1Kun/orion-live/internal/middleware"
	"github.com/X1Kun/orion-live/internal/service"
	"github.com/X1Kun/orion-live/pkg/logger"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	users service.UserService
}

type credentialsRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func NewUserHandler(users service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Register(c *gin.Context) {
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		sendError(c, http.StatusBadRequest, "INVALID_REQUEST", "username and password are required")
		return
	}

	user, err := h.users.Register(c.Request.Context(), request.Username, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameTaken):
			sendError(c, http.StatusConflict, "USERNAME_TAKEN", "username is already registered")
		case errors.Is(err, service.ErrInvalidUsername), errors.Is(err, service.ErrInvalidPassword):
			sendError(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		default:
			logger.Log.WithError(err).Error("register user")
			sendError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "request could not be completed")
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": user.ID, "username": user.Username}})
}

func (h *UserHandler) Login(c *gin.Context) {
	var request credentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		sendError(c, http.StatusBadRequest, "INVALID_REQUEST", "username and password are required")
		return
	}

	token, err := h.users.Login(c.Request.Context(), request.Username, request.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			sendError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "username or password is incorrect")
			return
		}
		logger.Log.WithError(err).Error("login user")
		sendError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "request could not be completed")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"access_token": token, "token_type": "Bearer"}})
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get(middleware.UserIDKey)
	if !exists {
		sendError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"user_id": userID}})
}

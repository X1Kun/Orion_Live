package handler

import "github.com/gin-gonic/gin"

// ErrorResponse 定义了标准的API错误响应结构
type ErrorResponse struct {
	Error string `json:"error"`
}

// sendErrorResponse 是一个辅助函数，用于发送标准格式的错误响应
func sendErrorResponse(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, ErrorResponse{Error: message})
}

package handler

import (
	"Orion_Live/internal/service"
	"Orion_Live/pkg/logger"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type LikeHandler interface {
	LikeVideo(c *gin.Context)
	UnlikeVideo(c *gin.Context)
}

type likeHandler struct {
	LikeService service.LikeService
}

func NewLikeHandler(likeService service.LikeService) LikeHandler {
	return &likeHandler{LikeService: likeService}
}

// 视频点赞：1、从URL通过:video_id获取videoID 2、从认证后的context获取userID 3、执行点赞服务
func (h *likeHandler) LikeVideo(c *gin.Context) {
	// :video_id用来定位资源(Resource)，把它放在URL路径里，用c.Param()获取，而Body承载(Payload)
	// URL中取回的是str，统一转化为uint64
	videoIDstr := c.Param("video_id")
	videoID, err := strconv.ParseUint(videoIDstr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的视频ID")
		return
	}
	// context中的userID是jwt中获取的，先断言为float64，再和videoID一样，统一转化为uint64
	userIDFloat, exists := c.Get("userID")
	if !exists {
		// 理论上中间件会拦截，但防御性编程是好习惯
		sendErrorResponse(c, http.StatusUnauthorized, "用户未认证")
		return
	}
	userID := uint64(userIDFloat.(float64))

	logCtx := logger.Log.WithField("user_id", userID).WithField("video_id", videoID)

	err = h.LikeService.LikeVideo(userID, videoID)
	if err != nil {
		logCtx.WithError(err).Error("点赞失败")
		// 这里的 err 是 service 层返回的业务逻辑错误，可以安全地展示给用户
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	logCtx.Info("点赞成功")
	c.JSON(http.StatusOK, gin.H{"message": "点赞成功"})
}

// 视频点赞：1、从URL通过:video_id获取videoID 2、从认证后的context获取userID 3、执行点赞服务
func (h *likeHandler) UnlikeVideo(c *gin.Context) {
	// c.Param返回字符串
	videoIDstr := c.Param("video_id")
	videoID, err := strconv.ParseUint(videoIDstr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的视频ID")
		return
	}
	// context中的userID是jwt中获取的，并且要经过断言
	userIDFloat, exists := c.Get("userID")
	if !exists {
		sendErrorResponse(c, http.StatusUnauthorized, "用户未认证")
		return
	}
	userID := uint64(userIDFloat.(float64))

	logCtx := logger.Log.WithField("user_id", userID).WithField("video_id", videoID)
	// strconv.FormatUint接收的参数类型是uint64，并转化为10进制字符串
	err = h.LikeService.UnlikeVideo(userID, videoID)
	if err != nil {
		logCtx.WithError(err).Error("取消点赞失败")
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	logCtx.Info("取消点赞成功")
	c.JSON(http.StatusOK, gin.H{"message": "取消点赞成功"})
}

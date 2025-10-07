package handler

import (
	"Orion_Live/internal/dto"
	"Orion_Live/internal/service"
	"Orion_Live/pkg/logger"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type VideoHandler interface {
	CreateVideo(c *gin.Context)

	GetVideoByID(c *gin.Context)
	GetFeed(c *gin.Context)
}

type videoHandler struct {
	VideoService service.VideoService
}

func NewVideoHandler(videoService service.VideoService) VideoHandler {
	return &videoHandler{VideoService: videoService}
}

type CreateVideoRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

// 创建视频：1、提取URL的Body和context中的userID 2、service层发布视频 3、将返回的视频结构通过dto传回
func (h *videoHandler) CreateVideo(c *gin.Context) {
	var req CreateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.WithError(err).Error("发布视频参数解析失败")
		sendErrorResponse(c, http.StatusBadRequest, "无效的参数")
		return
	}
	// 因为context中的userID是从jwt中间件中解析的，jwt.MapClaims中的数字相关会自动解析为float64，而context中的值又会被转化为interface{}
	userIDFloat, exists := c.Get("userID")
	// 防御性编程，其实正常肯定是jwt之后再创建视频的，但是就怕程序员误用
	if !exists {
		sendErrorResponse(c, http.StatusUnauthorized, "用户未认证")
		return
	}
	authorID := uint64(userIDFloat.(float64))
	// 蛇形命名法（日志聚合平台ELK、前端JavaScript）
	logCtx := logger.Log.WithField("author_id", authorID)
	logCtx.Info("开始处理发布视频请求")

	video, err := h.VideoService.CreateVideo(authorID, req.Title, req.Description)
	if err != nil {
		logCtx.WithError(err).Error("发布视频业务处理失败")
		sendErrorResponse(c, http.StatusInternalServerError, "发布视频失败")
		return
	}
	// 没有赋值，临时追加上下文，避免污染后续其他日志
	logCtx.WithField("video_id", video.ID).Info("视频发布成功")

	// 使用DTO转换函数，来构建一个干净、安全的响应
	response := dto.ToVideoResponse(video)

	c.JSON(http.StatusCreated, gin.H{ // 使用201 Created状态码，更符合RESTful规范
		"message": "视频发布成功",
		"data":    response,
	})

}

func (h *videoHandler) GetVideoByID(c *gin.Context) {
	videoIDstr := c.Param("video_id")
	videoID, err := strconv.ParseUint(videoIDstr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的视频ID")
		return
	}
	logCtx := logger.Log.WithField("video_id", videoID)
	logCtx.Info("开始处理查找视频请求")
	video, err := h.VideoService.GetVideoByID(videoID)
	if err != nil {
		// GetVideoByID 失败通常意味着资源不存在
		logCtx.WithError(err).Warn("查找视频失败")
		sendErrorResponse(c, http.StatusNotFound, "视频不存在")
		return
	}

	response := dto.ToVideoResponse(video)
	c.JSON(http.StatusOK, gin.H{"data": response})
}

// 可以无限向下滑动、不断出现新内容的主界面，就是最典型的Feed流，就是视频的元数据
// 获取视频Feed流：1、将请求附上用户IP，进行问题溯源 2、通过service层请求Feed流 3.dto层借助视频响应结构正确安全地返回大量Feed流
func (h *videoHandler) GetFeed(c *gin.Context) {
	// 攻击溯源，用户分析，问题排查
	logCtx := logger.Log.WithField("ip", c.ClientIP())
	logCtx.Info("开始处理获取Feed流请求")

	videos, err := h.VideoService.GetFeed(20)
	if err != nil {
		logCtx.WithError(err).Error("获取Feed流业务处理失败")
		sendErrorResponse(c, http.StatusInternalServerError, "获取视频流失败")
		return
	}

	// 将数据库模型列表转换为API响应模型列表
	var response []dto.VideoResponse
	for _, video := range videos {
		response = append(response, dto.ToVideoResponse(&video))
	}

	logCtx.WithField("count", len(response)).Info("成功获取Feed流")
	c.JSON(http.StatusOK, gin.H{
		"message": "成功获取视频流",
		"data":    response,
	})
}

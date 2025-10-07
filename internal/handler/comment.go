package handler

import (
	"Orion_Live/internal/dto"
	"Orion_Live/internal/repository"
	"Orion_Live/internal/service"
	"Orion_Live/pkg/logger"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CommentHandler interface {
	CreateCommentForVideo(c *gin.Context)
	CreateReplyForComment(c *gin.Context)
	CreateGoldenForVideo(c *gin.Context)

	GetComments(c *gin.Context)
}

type commentHandler struct {
	CommentService service.CommentService
	CommentRepo    repository.CommentRepository
	VideoRepo      repository.VideoRepository
}

func NewCommentHandler(commentService service.CommentService, commentRepo repository.CommentRepository, videoRepo repository.VideoRepository) CommentHandler {
	return &commentHandler{
		CommentService: commentService,
		CommentRepo:    commentRepo,
		VideoRepo:      videoRepo,
	}
}

type CreateCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// 视频评论：1、解析URL中的videoID参数，并判断视频是否存在 2、解析URL的Body，进行Content格式匹配 3、获取context中的userID（jwt） 4、创建评论并返回状态
func (h *commentHandler) CreateCommentForVideo(c *gin.Context) {
	// URL解析参数获得string格式
	videoIDstr := c.Param("video_id")
	// 利用strconv.ParseUint将string转化为uint64
	videoID, err := strconv.ParseUint(videoIDstr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的视频ID")
		return
	}
	_, err = h.VideoRepo.FindByID(videoID)
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "视频不存在")
		return
	}

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.WithError(err).Error("评论参数解析失败")
		sendErrorResponse(c, http.StatusBadRequest, "无效的参数") // 400
		return
	}

	// 因为context中的userID是从jwt中间件中解析的，jwt.MapClaims中的数字相关会自动解析为float64，而context中的值又会被转化为interface{}
	userIDFloat, exists := c.Get("userID")
	// 防御性编程，其实正常肯定是jwt之后再创建视频的，但是就怕程序员误用
	if !exists {
		sendErrorResponse(c, http.StatusUnauthorized, "用户未认证") // 401
		return
	}
	// 先断言，再转类型
	userID := uint64(userIDFloat.(float64))

	// 正式进入业务前，将logger格式整理好
	logCtx := logger.Log.WithField("user_id", userID).WithField("video_id", videoID)
	logCtx.Info("开始创建一级评论")
	comment, err := h.CommentService.CreateComment(userID, videoID, req.Content)
	if err != nil {
		logCtx.WithError(err).Error("创建一级评论失败")
		sendErrorResponse(c, http.StatusInternalServerError, "评论失败") // 500
		return
	}
	commentResponse := dto.ToCommentResponse(comment)
	// 业务成功，打上返回的comment的ID
	logCtx.WithField("comment_id", comment.ID).Info("一级评论创建成功")
	c.JSON(http.StatusCreated, gin.H{ //201
		"message": "评论成功",
		"data":    commentResponse, // MVP阶段可以直接返回model，未来换成DTO
	})

}

// 回复评论和正常评论的区别是，要查找回复的评论是否存在，以及创建的结构体需要含有父评论的ID，以及父评论的UserID也就是要回复的UserID
// 回复评论：1、提取URL中commentID参数，验证并提取父评论 2、从URL中的Body中取content，context中取userID 3、创建二级评论
func (h *commentHandler) CreateReplyForComment(c *gin.Context) {
	// c.Param从URL提取的commentID是str类型，转成uint64
	parentID, err := strconv.ParseUint(c.Param("comment_id"), 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的父评论ID") // 400
		return
	}
	// 确认父评论的存在,并返回父评论，供新回复建立结构体
	parentComment, err := h.CommentRepo.FindByID(parentID)
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "回复的评论不存在") // 404
		return
	}
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.WithError(err).Error("回复评论参数解析失败")
		sendErrorResponse(c, http.StatusBadRequest, "无效的参数") // 400
		return
	}

	// 从context中取得interface{}类型的userID
	userIDFloat, exists := c.Get("userID")
	// 防御性编程
	if !exists {
		sendErrorResponse(c, http.StatusUnauthorized, "用户未认证") // 401
		return
	}
	userID := uint64(userIDFloat.(float64))

	// 所有检查工作都做完，打上关键标签
	logCtx := logger.Log.WithField("user_id", userID).WithField("parent_id", parentID)
	logCtx.Info("开始创建二级评论")
	// 创建回复，加了父评论的信息
	reply, err := h.CommentService.CreateReply(userID, parentComment, req.Content)
	if err != nil {
		logCtx.WithError(err).Error("创建二级评论失败")
		sendErrorResponse(c, http.StatusBadRequest, err.Error()) // 400
		return
	}
	replyResponse := dto.ToReplyResponse(reply)
	// 业务成功，打上返回的comment的ID
	logCtx.WithField("reply_id", reply.ID).Info("二级评论创建成功")
	c.JSON(http.StatusCreated, gin.H{ //201
		"message": "回复成功",
		"data":    replyResponse,
	})
}

// 创建黄金评论：1、检查URL的video_id参数，以及video存在性 2、URL的Body参数嵌入，并从context提取userID 3、service层创建黄金评论，并返回 4.dto层处理返回的评论结构体，返回响应
func (h *commentHandler) CreateGoldenForVideo(c *gin.Context) {
	// 解析参数
	videoID, err := strconv.ParseUint(c.Param("video_id"), 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的视频ID") // 400
		return
	}
	// 查找videoID的存在性
	_, err = h.VideoRepo.FindByID(videoID)
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "视频不存在")
		return
	}
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.WithError(err).Error("评论参数解析失败")
		sendErrorResponse(c, http.StatusBadRequest, "无效的参数") // 400
		return
	}

	// 因为context中的userID是从jwt中间件中解析的，jwt.MapClaims中的数字相关会自动解析为float64，而context中的值又会被转化为interface{}
	userIDFloat, exists := c.Get("userID")
	// 防御性编程，其实正常肯定是jwt之后再创建视频的，但是就怕程序员误用
	if !exists {
		sendErrorResponse(c, http.StatusUnauthorized, "用户未认证") // 401
		return
	}
	// 先断言，再转类型
	userID := uint64(userIDFloat.(float64))

	// 正式进入业务前，将logger格式整理好
	logCtx := logger.Log.WithField("user_id", userID).WithField("video_id", videoID)
	logCtx.Info("开始创建黄金评论！")
	comment, err := h.CommentService.CreateGoldenComment(userID, videoID, req.Content)
	if err != nil {
		logCtx.WithError(err).Error("创建黄金评论失败")
		sendErrorResponse(c, http.StatusInternalServerError, "评论失败") // 500
		return
	}
	commentResponse := dto.ToCommentResponse(comment)
	// 业务成功，打上返回的comment的ID
	logCtx.WithField("comment_id", comment.ID).Info("黄金评论创建成功")
	c.JSON(http.StatusCreated, gin.H{ //201
		"message": "评论成功",
		"data":    commentResponse, // MVP阶段可以直接返回model，未来换成DTO
	})

}

// 获取一个视频的所有评论 1、提取URL中videoID参数，并确认存在 2、从查询参数获取分页信息，并提供默认值 3、通过service获取所有一级二级评论 4.dto层挂载二级评论，返回结果
func (h *commentHandler) GetComments(c *gin.Context) {
	// 解析参数
	videoID, err := strconv.ParseUint(c.Param("video_id"), 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的视频ID") // 400
		return
	}
	// 查找videoID的存在性
	_, err = h.VideoRepo.FindByID(videoID)
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, "视频不存在")
		return
	}
	// 在URL的查询参数里（?后面的部分）找page这个键，没找到就返回默认值“1”
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	// 调用Service获取所有一级评论和二级评论
	parentComments, replyMap, err := h.CommentService.GetComments(videoID, page, pageSize)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "获取评论列表失败") // 500
		return
	}
	response := dto.ToCommentResponses(parentComments, replyMap)

	c.JSON(http.StatusOK, gin.H{
		"message": "获取评论列表成功",
		"data":    response,
	})
}

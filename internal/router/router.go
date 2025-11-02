package router

import (
	"Orion_Live/internal/handler"
	"Orion_Live/internal/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRouter(userHandler handler.UserHandler, videoHandler handler.VideoHandler, likeHandler handler.LikeHandler, commentHandler handler.CommentHandler, wsHandler handler.WebSocketHandler) *gin.Engine {
	r := gin.Default()

	// 全局应用Metrics中间件
	r.Use(middleware.Metrics())

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pang",
		})
	})
	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/feed", videoHandler.GetFeed)
		apiV1.GET("/videos/:video_id", videoHandler.GetVideoByID)
		apiV1.GET("/videos/:video_id/comments", commentHandler.GetComments)
		// 实时弹幕
		apiV1.GET("/ws/videos/:video_id", wsHandler.ServeWs)

		userGroup := apiV1.Group("/users")
		{
			userGroup.POST("/register", userHandler.Register)
			userGroup.POST("/login", userHandler.Login)
		}

		authorized := apiV1.Group("/")
		authorized.Use(middleware.AuthMiddleware())
		{
			authorized.GET("/profile", userHandler.GetProfile)
			authorized.POST("/videos", videoHandler.CreateVideo)

			authorized.POST("/videos/:video_id/like", likeHandler.LikeVideo)
			authorized.DELETE("/videos/:video_id/like", likeHandler.UnlikeVideo)

			authorized.POST("/videos/:video_id/comments", commentHandler.CreateCommentForVideo)
			authorized.POST("/comments/:comment_id/replies", commentHandler.CreateReplyForComment)

			authorized.POST("/videos/:video_id/golden_comment", commentHandler.CreateGoldenForVideo)

		}
	}

	return r
}

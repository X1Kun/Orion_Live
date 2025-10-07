package router

import (
	"Orion_Live/internal/handler"
	"Orion_Live/internal/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRouter(userHandler handler.UserHandler, videoHandler handler.VideoHandler, likeHandler handler.LikeHandler, commentHandler handler.CommentHandler) *gin.Engine {
	r := gin.Default()
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

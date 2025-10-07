package dto

import (
	"Orion_Live/internal/model"
	"time"
)

type VideoResponse struct {
	ID          uint64    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	VideoURL    string    `json:"video_url"`
	CoverURL    string    `json:"cover_url"`
	Author      struct {  // 在这里定义了Author的精确形状
		ID       uint64 `json:"id"`
		Username string `json:"username"`
	} `json:"author"`
}

// ToVideoResponse 是一个转换函数，把DB模型转换为API响应模型，并且正确利用preload返回的数据，增强返回数据的健壮性
func ToVideoResponse(video *model.Video) VideoResponse {
	resp := VideoResponse{
		ID:          video.ID,
		CreatedAt:   video.CreatedAt,
		Title:       video.Title,
		Description: video.Description,
		VideoURL:    video.VideoURL,
		CoverURL:    video.CoverURL,
	}
	// 检查Author是否被成功preload
	if video.Author.ID != 0 {
		resp.Author.ID = video.Author.ID
		resp.Author.Username = video.Author.Username
	} else {
		// 如果没有preload，就返回video结构体本身的
		resp.Author.ID = video.AuthorID
	}
	return resp
}

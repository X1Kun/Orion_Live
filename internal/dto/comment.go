package dto

import (
	"Orion_Live/internal/model"
	"time"
)

// UserInfo 是在DTO中使用的、简化的用户信息
type UserInfo struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	// 未来可以加 Avatar string `json:"avatar"`
}

// ReplyResponse 是二级评论的响应结构
type ReplyResponse struct {
	ID        uint64    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	LikeCount uint64    `json:"like_count"`
	IsGolden  bool      `json:"is_golden"`
	Author    UserInfo  `json:"author"`
	ReplyTo   UserInfo  `json:"reply_to"` // 回复给了谁
}

// CommentResponse 是一级评论的响应结构，它包含了二级评论列表
type CommentResponse struct {
	ID        uint64          `json:"id"`
	Content   string          `json:"content"`
	CreatedAt time.Time       `json:"created_at"`
	LikeCount uint64          `json:"like_count"`
	IsGolden  bool            `json:"is_golden"`
	Author    UserInfo        `json:"author"`
	Replies   []ReplyResponse `json:"replies"` // 二级评论列表
}

func ToCommentResponse(comments *model.Comment) *CommentResponse {
	commentResponse := &CommentResponse{
		ID:        comments.ID,
		Content:   comments.Content,
		CreatedAt: comments.CreatedAt,
		LikeCount: comments.LikeCount,
		IsGolden:  comments.IsGolden,
	}
	if comments.User.ID != 0 {
		commentResponse.Author = UserInfo{
			ID:       comments.User.ID,
			Username: comments.User.Username,
		}
	}
	return commentResponse
}

func ToReplyResponse(reply *model.Comment) *ReplyResponse {
	replyResponse := &ReplyResponse{
		ID:        reply.ID,
		Content:   reply.Content,
		CreatedAt: reply.CreatedAt,
		LikeCount: reply.LikeCount,
		IsGolden:  reply.IsGolden,
	}
	if reply.User.ID != 0 {
		replyResponse.Author = UserInfo{
			ID:       reply.User.ID,
			Username: reply.User.Username,
		}
	}
	if reply.ReplyToUser.ID != 0 {
		replyResponse.ReplyTo = UserInfo{
			ID:       reply.ReplyToUser.ID,
			Username: reply.ReplyToUser.Username,
		}
	}
	return replyResponse
}

// ToCommentResponse 是我们的核心转换函数
// 它接收一个“一级评论”模型和它对应的“二级评论”模型列表
func ToCommentResponses(parentComments []model.Comment, groupReplies map[uint64][]*model.Comment) []CommentResponse {

	// 创建一个有预估容量的切片，性能稍好
	response := make([]CommentResponse, 0, len(parentComments))

	for _, pc := range parentComments {
		commentResp := CommentResponse{
			ID:        pc.ID,
			Content:   pc.Content,
			CreatedAt: pc.CreatedAt,
			LikeCount: pc.LikeCount,
			IsGolden:  pc.IsGolden,
			// 这种不安全，需要单独地安全填充作者信息
			// Author: UserInfo{
			// 	ID:       pc.UserID,
			// 	Username: pc.User.Username,
			// },
			Replies: []ReplyResponse{},
		}
		// 安全地填充作者信息，我还想质疑ID是否可能为0，但是大模型告诉我MySQL的AUTO_INCREMENT默认就是从1开始的
		if pc.User.ID != 0 {
			commentResp.Author = UserInfo{
				ID:       pc.User.ID,
				Username: pc.User.Username,
			}
		}
		// 查找该一级评论对应的二级评论列表
		if replies, ok := groupReplies[pc.ID]; ok {
			for _, r := range replies {
				replyResp := ReplyResponse{
					ID:        r.ID,
					Content:   r.Content,
					CreatedAt: r.CreatedAt,
					LikeCount: r.LikeCount,
					IsGolden:  r.IsGolden,
				}
				// 安全地填充二级评论的作者
				if r.User.ID != 0 {
					replyResp.Author = UserInfo{
						ID:       r.User.ID,
						Username: r.User.Username,
					}
				}
				// 安全地填充被回复者信息
				if r.ReplyToUser.ID != 0 {
					replyResp.ReplyTo = UserInfo{
						ID:       r.ReplyToUser.ID,
						Username: r.ReplyToUser.Username,
					}
				}
				commentResp.Replies = append(commentResp.Replies, replyResp)
			}
		}
		response = append(response, commentResp)
	}

	return response
}

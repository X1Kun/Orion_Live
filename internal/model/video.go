package model

// Video结构，视频都要有什么？比如b站的视频，up主（作者），标题，简介
type Video struct {
	BaseModel
	AuthorID    uint64 `gorm:"not null"` // 作者ID，用于关联用户
	Title       string `gorm:"not null"` // 视频标题
	Description string // 视频简介
	LikeCount   uint64 `gorm:"default:0"`
	GoldenCount uint64 `gorm:"default:0"`

	VideoURL string `gorm:"not null"` // 视频播放地址
	CoverURL string `gorm:"not null"` // 视频封面地址

	// 外键AuthorID和User表的ID
	Author User `gorm:"foreignKey:AuthorID;references:ID"`
}

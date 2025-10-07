package model

type Comment struct {
	BaseModel
	VideoID uint64 `gorm:"not null;index"` // index索引，极大地加速基于该列的查询、过滤和排序操作
	UserID  uint64 `gorm:"not null;index"`
	// TEXT是MySQL中的一种文本类型，专门用于存储非常长的字符串，最大长度可达65,535个字符
	Content   string `gorm:"type:text;not null"`
	LikeCount uint64 `gorm:"default:0"`
	IsGolden  bool   `gorm:"default:false"`
	// 指针*uint64的零值是nil，这样就可以区分是一级评论还是二级评论
	ParentID      *uint64 `gorm:"index"`
	ReplyToUserID *uint64

	User        User `gorm:"foreignKey:UserID"`
	ReplyToUser User `gorm:"foreignKey:ReplyToUserID"`
}

func (Comment) TableName() string {
	return "comments"
}

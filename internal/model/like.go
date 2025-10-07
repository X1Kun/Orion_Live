package model

// 用户与视频的关联关系，uniqueIndex利用的是MySQL数据库的“自动查重”能力，而不是gorm的
type Like struct {
	BaseModel
	UserID  uint64 `gorm:"uniqueIndex:idx_user_video"` // 设置联合唯一索引
	VideoID uint64 `gorm:"uniqueIndex:idx_user_video"` // 确保一个用户对一个视频只能点赞一次

}

// 想精确控制表名，或表名不符合GORM的复数规则，就必须实现TableName()方法规定表名
func (Like) TableName() string {
	return "likes"
}

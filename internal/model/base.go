package model

import (
	"time"

	"gorm.io/gorm"
)

// 由于gorm的基本结构中ID是uint类型，我想都统一成uint64，所以自己搞了个base结构体
type BaseModel struct {
	// 由于在慢查询的分析过程中，发现像/feed这样需要经常性获取最新视频，也就是对创建时间进行排序的过程贼慢，所以直接加个，效果显著
	ID        uint64    `gorm:"primarykey"`
	CreatedAt time.Time `gorm:"index"`
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

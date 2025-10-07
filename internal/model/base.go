package model

import (
	"time"

	"gorm.io/gorm"
)

// 由于gorm的基本结构中ID是uint类型，我想都统一成uint64，所以自己搞了个base结构体
type BaseModel struct {
	ID        uint64 `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

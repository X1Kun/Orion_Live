package model

import (
	"time"
)

type BaseModel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time
}

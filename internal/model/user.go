package model

type User struct {
	BaseModel        // 包括 ID, CreatedAt, UpdatedAt, DeleteAt
	Username  string `gorm:"unique;not null"`
	Password  string `gorm:"not null"`
}

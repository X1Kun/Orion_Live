package model

type User struct {
	BaseModel
	Username     string `gorm:"size:64;uniqueIndex;not null"`
	PasswordHash string `gorm:"size:60;not null"`
}

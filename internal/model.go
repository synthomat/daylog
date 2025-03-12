package internal

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type BaseModel struct {
	Id        uuid.UUID      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gson:"default:CURRENT_TIMESTAMP;not null" json:"created_at"`
	UpdatedAt *time.Time     `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at"`
}

func (u *BaseModel) BeforeCreate(tx *gorm.DB) (err error) {
	u.Id = uuid.New()
	return
}

type Post struct {
	BaseModel
	EventTime time.Time `gorm:"datetime" json:"event_time"`
	Title     *string   `json:"title"`
	Body      string    `json:"body"`
}

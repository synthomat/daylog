package internal

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	Id        uuid.UUID      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"not null" json:"createdAt"`
	UpdatedAt *time.Time     `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"deletedAt"`
}

func (u *BaseModel) BeforeCreate(_ *gorm.DB) (err error) {
	u.Id = uuid.New()
	return
}

type Post struct {
	BaseModel
	EventTime time.Time `gorm:"datetime" json:"eventTime"`
	Title     *string   `json:"title"`
	Body      string    `json:"body"`
}

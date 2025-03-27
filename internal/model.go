package internal

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type BaseModel struct {
	Id        uuid.UUID      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"default:CURRENT_TIMESTAMP;not null" json:"createdAt"`
	UpdatedAt *time.Time     `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"deletedAt"`
}

func (u *BaseModel) BeforeCreate(tx *gorm.DB) (err error) {
	u.Id = uuid.New()
	u.CreatedAt = time.Now()
	return
}

type Post struct {
	BaseModel
	EventTime time.Time `gorm:"datetime" json:"eventTime"`
	Title     *string   `json:"title"`
	Body      string    `json:"body"`
}

type Attachment struct {
	Id        uuid.UUID `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;not null" json:"createdAt"`

	PostId   uuid.UUID `gorm:"primaryKey" json:"postId"`
	Post     *Post     `gorm:"foreignKey:PostId" json:"-"`
	InUse    bool      `gorm:"default:false" json:"inUse"`
	FilePath string    `json:"filePath"`
	FileHash string    `json:"fileHash"`
}

func (u *Attachment) BeforeCreate(tx *gorm.DB) (err error) {
	u.Id = uuid.New()
	u.CreatedAt = time.Now()
	return
}

package internal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
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

type Files []string

func (f Files) Scan(value interface{}) error {
	err := json.Unmarshal([]byte(value.(string)), &f)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func (f Files) Value() (driver.Value, error) {
	sse, _ := json.Marshal([]string(f))

	return string(sse), nil
}

type Attachment struct {
	Id        uuid.UUID `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;not null" json:"createdAt"`

	PostId *uuid.UUID `gorm:"primaryKey" json:"postId"`
	Post   *Post      `gorm:"foreignKey:PostId" json:"-"`
	//FilePath string     `json:"filePath"`
	Files Files `gorm:"type:TEXT"`

	OriginalFileName string `gorm:"type:TEXT;not null" json:"originalFileName"`
}

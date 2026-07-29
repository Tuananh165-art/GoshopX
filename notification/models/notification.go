package models

import "time"

type Notification struct {
	ID           uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	AccountID    uint64     `gorm:"index" json:"account_id"`
	EventID      string     `gorm:"uniqueIndex;size:128" json:"event_id"`
	EventType    string     `gorm:"size:128" json:"event_type"`
	Title        string     `gorm:"size:255" json:"title"`
	Message      string     `gorm:"type:text" json:"message"`
	MetadataJSON string     `gorm:"type:text" json:"metadata_json"`
	IsRead       bool       `json:"is_read"`
	CreatedAt    time.Time  `json:"created_at"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
}

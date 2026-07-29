package models

import "time"

type InventoryEventOutbox struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	EventKey  string    `gorm:"uniqueIndex;size:128"`
	EventType string    `gorm:"size:128"`
	Payload   string    `gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

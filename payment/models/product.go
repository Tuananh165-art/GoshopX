package models

import (
	"time"
)

type Product struct {
	ID            uint64 `json:"id" gorm:"primarykey;autoIncrement"`
	ProductID     string `json:"productId"`
	DodoProductID string `json:"dodoProductId"`
	Price         int64  `json:"price"`
	Currency      string `json:"currency"`

	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;"`
}

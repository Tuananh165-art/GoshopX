package models

import "time"

type Stock struct {
	ProductID    string    `gorm:"primaryKey;size:128" json:"product_id"`
	Quantity     int32     `json:"quantity"`
	ReorderLevel int32     `json:"reorder_level"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Availability struct {
	ProductID          string `json:"product_id"`
	TotalQuantity      int32  `json:"total_quantity"`
	ReservedQuantity   int32  `json:"reserved_quantity"`
	AvailableQuantity  int32  `json:"available_quantity"`
	ReorderLevel       int32  `json:"reorder_level"`
	HasActiveLowStock  bool   `json:"has_active_low_stock"`
}

package models

import "time"

type ReservationStatus string

const (
	ReservationActive    ReservationStatus = "active"
	ReservationReleased  ReservationStatus = "released"
	ReservationCommitted ReservationStatus = "committed"
	ReservationExpired   ReservationStatus = "expired"
)

type StockReservation struct {
	ID            uint              `gorm:"primaryKey;autoIncrement"`
	ReservationID string            `gorm:"uniqueIndex;size:64" json:"reservation_id"`
	AccountID     uint64            `json:"account_id"`
	ProductID     string            `gorm:"index;size:128" json:"product_id"`
	Quantity      int32             `json:"quantity"`
	Status        ReservationStatus `gorm:"type:varchar(20)" json:"status"`
	Source        string            `gorm:"type:varchar(32)" json:"source"`
	ExpiresAt     time.Time         `json:"expires_at"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

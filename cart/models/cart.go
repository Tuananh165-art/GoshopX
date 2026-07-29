package models

import "time"

type Cart struct {
	AccountID uint64     `json:"account_id"`
	Items     []CartItem `json:"items"`
	ExpiresAt time.Time  `json:"expires_at"`
}

type CartItem struct {
	ProductID     string    `json:"product_id"`
	Quantity      int32     `json:"quantity"`
	ReservationID string    `json:"reservation_id"`
	ReservedUntil time.Time `json:"reserved_until"`
}

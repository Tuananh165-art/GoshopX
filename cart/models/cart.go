package models

import "time"

type Cart struct {
	AccountID uint64     `json:"account_id"`
	Version   uint64     `json:"version"`
	Items     []CartItem `json:"items"`
	ExpiresAt time.Time  `json:"expires_at"`
}

type CartItem struct {
	ProductID     string    `json:"product_id"`
	Quantity      int32     `json:"quantity"`
	ReservationID string    `json:"reservation_id"`
	ReservedUntil time.Time `json:"reserved_until"`
}

type ProposalItem struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

type CartProposal struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key"`
	AccountID      uint64         `json:"account_id"`
	BaseVersion    uint64         `json:"base_version"`
	Items          []ProposalItem `json:"items"`
	ExpiresAt      time.Time      `json:"expires_at"`
	Result         *Cart          `json:"result,omitempty"`
}

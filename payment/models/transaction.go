package models

import "gorm.io/gorm"

type TransactionStatus string

const (
	Pending = TransactionStatus("Pending")
	Failed  = TransactionStatus("Failed")
	Success = TransactionStatus("Success")
)

func (s TransactionStatus) String() string { return string(s) }
func (s TransactionStatus) Terminal() bool { return s == Failed || s == Success }

type Transaction struct {
	gorm.Model
	OrderId        uint64 `json:"order_id"`
	UserId         uint64 `json:"user_id"`
	PaymentId      string `json:"payment_id" gorm:"uniqueIndex"`
	TotalPrice     int64  `json:"total_price"`
	SettledPrice   int64  `json:"settled_price"`
	Currency       string `json:"currency"`
	Status         string `json:"status" gorm:"type:varchar(20);index"`
	ReservationIDs string `json:"reservation_ids" gorm:"type:text"`
}
type Refund struct {
	gorm.Model
	TransactionID    uint   `gorm:"index" json:"transaction_id"`
	PaymentID        string `gorm:"index" json:"payment_id"`
	ProviderRefundID string `gorm:"uniqueIndex" json:"provider_refund_id"`
	IdempotencyKey   string `gorm:"uniqueIndex;size:128" json:"idempotency_key"`
	Amount           int64  `json:"amount"`
	Reason           string `gorm:"type:text" json:"reason"`
	Status           string `gorm:"type:varchar(20)" json:"status"`
}

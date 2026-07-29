package models

import (
	"gorm.io/gorm"
)

type TransactionStatus string

const (
	Failed  = TransactionStatus("Failed")
	Success = TransactionStatus("Success")
)

func (s TransactionStatus) String() string {
	return string(s)
}

type Transaction struct {
	gorm.Model
	OrderId      uint64 `json:"order_id"`
	UserId       uint64 `json:"user_id"`
	CustomerId   string `json:"customer_id"`
	PaymentId    string `json:"payment_id"`
	TotalPrice   int64  `json:"total_price"`
	SettledPrice int64  `json:"settled_price"`
	Currency     string `json:"currency"`
	Status       string `json:"status" gorm:"type:varchar(20)"`
}

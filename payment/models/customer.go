package models

import "time"

type Customer struct {
	UserId       uint64    `json:"user_id" gorm:"primary_key"`
	CustomerId   string    `json:"customer_id"`
	BillingEmail string    `json:"billing_email"`
	BillingName  string    `json:"billing_name"`
	CreatedAt    time.Time `json:"created_at"`

	Transactions []Transaction `json:"transactions"`
}

type CustomerInput struct {
	UserId string `json:"user_id"`
}

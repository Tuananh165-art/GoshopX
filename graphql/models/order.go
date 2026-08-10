package models

import "time"

type Order struct {
	ID         int               `json:"id"`
	CreatedAt  time.Time         `json:"createdAt"`
	TotalPrice float64           `json:"totalPrice"`
	Products   []*OrderedProduct `json:"products"`
}

type OrderedProduct struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

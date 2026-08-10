package models

type ProductEventData struct {
	ProductID   *string  `json:"product_id"`
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	AccountID   *int     `json:"accountID"`
}

type ProductEvent struct {
	Type string           `json:"type"`
	Data ProductEventData `json:"data"`
}

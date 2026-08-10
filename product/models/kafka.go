package models

type EventData struct {
	ID          *string  `json:"product_id"`
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	AccountID   *int     `json:"accountID"`
}

type Event struct {
	Type string    `json:"type"`
	Data EventData `json:"data"`
}

package dto

// EventData mirrors product/models.EventData used by product service Kafka messages
// We only care about product creation events here.
type EventData struct {
	ID          *string  `json:"product_id"`
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	AccountID   *int     `json:"accountID"`
}

// Event mirrors product/models.Event
type Event struct {
	Type string    `json:"type"`
	Data EventData `json:"data"`
}

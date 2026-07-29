package models

type EventData struct {
	ProductID      string   `json:"product_id,omitempty"`
	Quantity       int32    `json:"quantity,omitempty"`
	ReservationID  string   `json:"reservation_id,omitempty"`
	ReservationIDs []string `json:"reservation_ids,omitempty"`
	CartSize       int      `json:"cart_size,omitempty"`
	Action         string   `json:"action,omitempty"`
}

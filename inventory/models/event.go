package models

type EventData struct {
	ProductID      string `json:"product_id"`
	ReservationID  string `json:"reservation_id,omitempty"`
	Quantity       int32  `json:"quantity,omitempty"`
	ReorderLevel   int32  `json:"reorder_level,omitempty"`
	AvailableQty   int32  `json:"available_quantity,omitempty"`
	ReservedQty    int32  `json:"reserved_quantity,omitempty"`
	ReservationFor string `json:"source,omitempty"`
	Status         string `json:"status,omitempty"`
}

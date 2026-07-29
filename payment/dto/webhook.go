package dto

import "github.com/dodopayments/dodopayments-go"

type WebhookMetadata struct {
	OrderId uint64 `json:"order_id"`
	UserId  uint64 `json:"user_id"`
}

type WebhookPayload struct {
	Type string `json:"type"`
	Data struct {
		Customer struct {
			CustomerID string `json:"customer_id"`
			Email      string `json:"email"`
			Name       string `json:"name"`
		} `json:"customer"`
		ProductCart []struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		} `json:"product_cart"` // Product cart is going to be a slice of one element since
		// we always pass one product with the quantity one
		PaymentId     string                `json:"payment_id"`
		Metadata      WebhookMetadata       `json:"metadata"`
		TotalAmount   int64                 `json:"total_amount"`
		SettledAmount int64                 `json:"settled_amount"`
		Currency      dodopayments.Currency `json:"currency"`
		Status        string                `json:"status"`
	} `json:"data"`
}

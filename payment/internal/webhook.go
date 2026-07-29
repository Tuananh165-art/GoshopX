package internal

import (
	"context"
	"log"
	"net/http"
	"time"

	order "github.com/rasadov/EcommerceAPI/order/client"
)

type WebhookServer struct {
	service     Service
	orderClient *order.Client
}

func (s *WebhookServer) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transaction, err := s.service.HandlePaymentWebhook(ctx, w, r)
	if err != nil {
		log.Println(err.Error())
		return
	}

	err = s.orderClient.UpdateOrderStatus(ctx, transaction.OrderId, transaction.Status)
	if err != nil {
		log.Println(err.Error())
	}
}

package internal

import (
	"context"
	"log"
	"net/http"
	"time"

	cart "github.com/Tuananh165art/GoshopX/cart/client"
	inventory "github.com/Tuananh165art/GoshopX/inventory/client"
	order "github.com/Tuananh165art/GoshopX/order/client"
	"github.com/Tuananh165art/GoshopX/payment/models"
)

type WebhookServer struct {
	service         Service
	orderClient     *order.Client
	inventoryClient *inventory.Client
	cartClient      *cart.Client
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

	for _, reservationID := range reservationIDs(transaction.ReservationIDs) {
		switch transaction.Status {
		case models.Success.String():
			if _, _, err := s.inventoryClient.CommitReservation(ctx, reservationID); err != nil {
				log.Println(err.Error())
			}
		default:
			if _, _, err := s.inventoryClient.ReleaseReservation(ctx, reservationID); err != nil {
				log.Println(err.Error())
			}
		}
	}

	if err := s.cartClient.ClearCart(ctx, transaction.UserId); err != nil {
		log.Println(err.Error())
	}
}


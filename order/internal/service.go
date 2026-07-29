package internal

import (
	"context"
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/order/config"
	"github.com/Tuananh165art/GoshopX/order/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
)

type Service interface {
	PostOrder(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct) (*models.Order, error)
	GetOrdersForAccount(ctx context.Context, accountID uint64) ([]*models.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, status string) error
	GetProducer() sarama.AsyncProducer
}

type orderService struct {
	repository Repository
	producer   sarama.AsyncProducer
}

func NewOrderService(repository Repository, producer sarama.AsyncProducer) Service {
	return &orderService{repository, producer}
}

func (service orderService) GetProducer() sarama.AsyncProducer {
	return service.producer
}

func (service orderService) PostOrder(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct) (*models.Order, error) {
	order := models.Order{
		AccountID:  accountID,
		TotalPrice: totalPrice,
		Products:   products,
		CreatedAt:  time.Now().UTC(),
	}
	err := service.repository.PutOrder(ctx, &order)
	if err != nil {
		return nil, err
	}

	// Send to recommendation service
	go func() {
		if err != nil {
			log.Println("Failed to convert account ID to int:", err)
			return
		}
		for _, product := range products {
			err = kafka.SendMessageToRecommender(service, models.Event{
				Type: "purchase",
				EventData: models.EventData{
					AccountId: int(accountID),
					ProductId: product.ID,
				},
			}, "interaction_events")
			if err != nil {
				log.Println("Failed to send event to recommendation service:", err)
			}
		}
	}()

	go func() {
		err = kafka.SendMessage(service, sharedevents.New("order.created", accountID, "order-created", map[string]any{
			"order_id":    order.ID,
			"account_id":  order.AccountID,
			"total_price": order.TotalPrice,
			"product_ids": productIDs(products),
		}), config.OrderEventsTopic)
		if err != nil {
			log.Println("Failed to send order event:", err)
		}
	}()

	return &order, nil
}

func (service orderService) GetOrdersForAccount(ctx context.Context, accountID uint64) ([]*models.Order, error) {
	return service.repository.GetOrdersForAccount(ctx, accountID)
}

func (service orderService) UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, paymnetStatus string) error {
	if err := service.repository.UpdateOrderPaymentStatus(ctx, orderId, paymnetStatus); err != nil {
		return err
	}
	go func() {
		err := kafka.SendMessage(service, sharedevents.New("order.payment_status_updated", 0, "order-status-updated", map[string]any{
			"order_id": orderId,
			"status":   paymnetStatus,
		}), config.OrderEventsTopic)
		if err != nil {
			log.Println("Failed to send order status event:", err)
		}
	}()
	return nil
}

func productIDs(products []*models.OrderedProduct) []string {
	ids := make([]string, 0, len(products))
	for _, product := range products {
		ids = append(ids, product.ID)
	}
	return ids
}


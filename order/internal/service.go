package internal

import (
	"context"
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/rasadov/EcommerceAPI/order/models"
	"github.com/rasadov/EcommerceAPI/pkg/kafka"
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

	return &order, nil
}

func (service orderService) GetOrdersForAccount(ctx context.Context, accountID uint64) ([]*models.Order, error) {
	return service.repository.GetOrdersForAccount(ctx, accountID)
}

func (service orderService) UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, paymnetStatus string) error {
	return service.repository.UpdateOrderPaymentStatus(ctx, orderId, paymnetStatus)
}

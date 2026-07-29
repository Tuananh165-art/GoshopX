package internal

import (
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
)

type EventConsumer struct {
	consumer sarama.Consumer
	service  Service
}

func NewEventConsumer(consumer sarama.Consumer, service Service) *EventConsumer {
	return &EventConsumer{
		consumer: consumer,
		service:  service,
	}
}

func (ec *EventConsumer) GetConsumer() sarama.Consumer {
	return ec.consumer
}

func (ec *EventConsumer) StartProductEventsConsumer(ctx context.Context) error {
	return kafka.StartEventsConsumer(ctx, ec, "product_events", ec.handleProductEvent)
}

func (ec *EventConsumer) handleProductEvent(partition int32, pc sarama.PartitionConsumer) {
	for {
		select {
		case message := <-pc.Messages():
			if message == nil {
				continue
			}

			var event models.ProductEvent
			if err := json.Unmarshal(message.Value, &event); err != nil {
				log.Printf("Failed to unmarshal product event: %v", err)
				continue
			}

			switch event.Type {
			case "product_created":
				ec.handleProductCreated(event)
			case "product_updated":
				ec.handleProductUpdated(event)
			case "product_deleted":
				ec.handleProductDeleted(event)
			default:
				log.Printf("Unknown event type: %s", event.Type)
			}

		case err := <-pc.Errors():
			if err != nil {
				log.Printf("Kafka consumer error: %v", err)
			}
		}
	}
}

func (ec *EventConsumer) handleProductCreated(event models.ProductEvent) {
	if event.Data.ProductID == nil || event.Data.Name == nil || event.Data.Price == nil {
		log.Printf("Invalid product created event: missing required fields")
		return
	}

	log.Printf("Payment service received product created event: ID=%s, Name=%s, Price=%.2f",
		*event.Data.ProductID, *event.Data.Name, *event.Data.Price)

	ctx := context.Background()
	err := ec.service.RegisterProduct(ctx, *event.Data.Name, int64(*event.Data.Price*100), "", *event.Data.ProductID)
	if err != nil {
		log.Printf("Failed to register product with payment provider: %v", err)
	}
}

func (ec *EventConsumer) handleProductUpdated(event models.ProductEvent) {
	if event.Data.ProductID == nil {
		log.Printf("Invalid product updated event: missing product ID")
		return
	}

	log.Printf("Payment service received product updated event: ID=%s", *event.Data.ProductID)
	ctx := context.Background()
	err := ec.service.UpdateProduct(ctx, *event.Data.ProductID, *event.Data.Name, int64(*event.Data.Price*100))
	if err != nil {
		log.Printf("Failed to update product with payment provider: %v", err)
	}
}

func (ec *EventConsumer) handleProductDeleted(event models.ProductEvent) {
	if event.Data.ProductID == nil {
		log.Printf("Invalid product deleted event: missing product ID")
		return
	}

	log.Printf("Payment service received product deleted event: ID=%s", *event.Data.ProductID)

	ctx := context.Background()
	err := ec.service.DeleteProduct(ctx, *event.Data.ProductID)
	if err != nil {
		log.Printf("Failed to delete product with payment provider: %v", err)
	}
}


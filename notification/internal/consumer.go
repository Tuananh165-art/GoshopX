package internal

import (
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/notification/config"
	"github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
)

type EventConsumer struct {
	consumer sarama.Consumer
	service  Service
}

func NewEventConsumer(consumer sarama.Consumer, service Service) *EventConsumer {
	return &EventConsumer{consumer: consumer, service: service}
}

func (consumer *EventConsumer) GetConsumer() sarama.Consumer {
	return consumer.consumer
}

func (consumer *EventConsumer) Start(ctx context.Context) error {
	topics := []string{config.CartTopic, config.InventoryTopic, config.OrderTopic, config.PaymentTopic, config.AccountTopic}
	for _, topic := range topics {
		topic := topic
		go func() {
			if err := kafka.StartEventsConsumer(ctx, consumer, topic, consumer.handleEvent); err != nil {
				log.Printf("notification consumer error on topic %s: %v", topic, err)
			}
		}()
	}
	<-ctx.Done()
	return nil
}

func (consumer *EventConsumer) handleEvent(partition int32, pc sarama.PartitionConsumer) {
	for {
		select {
		case message := <-pc.Messages():
			if message == nil {
				continue
			}
			var event events.Envelope
			if err := json.Unmarshal(message.Value, &event); err != nil {
				log.Printf("notification event unmarshal failed: %v", err)
				continue
			}
			if err := consumer.service.CreateFromEvent(context.Background(), &event); err != nil {
				log.Printf("notification event handling failed: %v", err)
			}
		case err := <-pc.Errors():
			if err != nil {
				log.Printf("notification partition error: %v", err)
			}
		}
	}
}

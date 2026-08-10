package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/pkg/events"
)

type Consumer struct {
	consumer   sarama.Consumer
	repository Repository
}

func NewConsumer(consumer sarama.Consumer, repository Repository) *Consumer {
	return &Consumer{consumer: consumer, repository: repository}
}
func (c *Consumer) GetConsumer() sarama.Consumer { return c.consumer }

func (c *Consumer) Start(ctx context.Context, topics []string) {
	for _, topic := range topics {
		partitions, err := c.consumer.Partitions(topic)
		if err != nil {
			log.Printf("admin topic unavailable topic=%s: %v", topic, err)
			continue
		}
		for _, partition := range partitions {
			pc, err := c.consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
			if err != nil {
				log.Printf("admin consume start failed topic=%s partition=%d: %v", topic, partition, err)
				continue
			}
			go c.ConsumePartition(ctx, topic, partition, pc)
		}
	}
}

func (c *Consumer) HandleMessage(ctx context.Context, topic string, message *sarama.ConsumerMessage) error {
	var event events.Envelope
	if err := json.Unmarshal(message.Value, &event); err != nil {
		_ = c.repository.Quarantine(ctx, topic, message.Partition, message.Offset, message.Value, "invalid event envelope: "+err.Error())
		return err
	}
	if event.Version != "v1" {
		err := fmt.Errorf("unsupported event version %q", event.Version)
		_ = c.repository.Quarantine(ctx, topic, message.Partition, message.Offset, message.Value, err.Error())
		return err
	}
	if err := c.repository.ApplyEvent(ctx, topic, event); err != nil {
		_ = c.repository.Quarantine(ctx, topic, message.Partition, message.Offset, message.Value, err.Error())
		return err
	}
	return nil
}

func (c *Consumer) ConsumePartition(ctx context.Context, topic string, partition int32, pc sarama.PartitionConsumer) {
	for {
		select {
		case message := <-pc.Messages():
			if message != nil {
				if err := c.HandleMessage(ctx, topic, message); err != nil {
					log.Printf("admin event failed topic=%s: %v", topic, err)
				}
			}
		case <-ctx.Done():
			return
		case err := <-pc.Errors():
			if err != nil {
				log.Printf("admin kafka error topic=%s: %v", topic, err)
			}
		}
	}
}

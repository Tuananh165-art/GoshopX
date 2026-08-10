package kafka

import (
	"context"
	"log"

	"github.com/IBM/sarama"
)

type ConsumerService interface {
	GetConsumer() sarama.Consumer
}

// StartEventsConsumer starts a simple Kafka consumer that listens to the given topic
func StartEventsConsumer(ctx context.Context, service ConsumerService, topic string, OnEvent func(p int32, pc sarama.PartitionConsumer)) error {
	partitions, err := service.GetConsumer().Partitions(topic)
	if err != nil {
		return err
	}

	log.Printf("Payment Kafka consumer starting; topic=%s partitions=%v", topic, partitions)

	done := make(chan struct{})
	for _, partition := range partitions {
		pc, err := service.GetConsumer().ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			log.Printf("Error starting partition consumer p=%d: %v", partition, err)
			continue
		}

		go OnEvent(partition, pc)
	}

	<-ctx.Done()
	close(done)
	return nil
}

func CloseConsumer(service ConsumerService) {
	if err := service.GetConsumer().Close(); err != nil {
		log.Printf("Failed to close consumer: %v\n", err)
	} else {
		done <- true
	}
}

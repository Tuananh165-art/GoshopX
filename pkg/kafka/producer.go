package kafka

import (
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
)

var done = make(chan bool)

type ProducerService interface {
	GetProducer() sarama.AsyncProducer
}

func SendMessage(service ProducerService, event any, topic string) error {
	producer := service.GetProducer()
	if producer == nil {
		log.Println("Kafka producer not configured, skipping message")
		return nil
	}

	jsonMessage, err := json.Marshal(event)
	if err != nil {
		log.Println("Failed to marshal event:", err)
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(jsonMessage),
	}

	producer.Input() <- msg
	return nil
}

func SendMessageToRecommender(service ProducerService, event any, topic string) error {
	return SendMessage(service, event, topic)
}

func CloseProducer(service ProducerService) {
	producer := service.GetProducer()
	if producer == nil {
		return
	}

	if err := producer.Close(); err != nil {
		log.Printf("Failed to close producer: %v\n", err)
	} else {
		done <- true
	}
}

package main

import (
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/payment/config"
	"github.com/Tuananh165art/GoshopX/payment/internal"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var repository internal.Repository
	var producer sarama.AsyncProducer
	var err error

	retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
		db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
		if err != nil {
			log.Println(err)
		}
		repository, err = internal.NewPostgresRepository(db)
		if err != nil {
			log.Println(err)
		}
		return
	})

	if config.KafkaBrokers != "" {
		producer, err = sarama.NewAsyncProducer([]string{config.KafkaBrokers}, nil)
		if err != nil {
			log.Printf("Failed to create Kafka producer: %v", err)
			producer = nil
		} else {
			defer func() {
				if err := producer.Close(); err != nil {
					log.Println(err)
				}
			}()
		}
	}

	// Setup Kafka consumer
	var consumer sarama.Consumer
	if config.KafkaBrokers != "" {
		retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
			kafkaConfig := sarama.NewConfig()
			kafkaConfig.Consumer.Return.Errors = true

			consumer, err = sarama.NewConsumer([]string{config.KafkaBrokers}, kafkaConfig)
			if err != nil {
				log.Printf("Failed to create Kafka consumer: %v", err)
			}
			return
		})
	}

	dodoClient := internal.NewDodoClient(config.DodoAPIKEY, config.DodoTestMode)
	service := internal.NewPaymentService(dodoClient, repository, producer)

	log.Fatal(internal.StartServers(service, consumer, config.OrderServiceURL, config.InventoryServiceURL, config.CartServiceURL, config.GrpcPort, config.WebhookPort))
}


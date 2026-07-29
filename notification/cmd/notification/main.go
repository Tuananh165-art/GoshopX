package main

import (
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/notification/config"
	"github.com/Tuananh165art/GoshopX/notification/internal"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var repository internal.Repository

	retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
		db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
		if err != nil {
			log.Println(err)
			return err
		}
		repository, err = internal.NewPostgresRepository(db)
		if err != nil {
			log.Println(err)
		}
		return err
	})
	defer repository.Close()

	var consumer sarama.Consumer
	if config.KafkaBrokers != "" {
		retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
			kafkaConfig := sarama.NewConfig()
			kafkaConfig.Consumer.Return.Errors = true
			consumer, err = sarama.NewConsumer([]string{config.KafkaBrokers}, kafkaConfig)
			if err != nil {
				log.Printf("Failed to create Kafka consumer: %v", err)
			}
			return err
		})
	}

	service := internal.NewNotificationService(repository)
	log.Println("Notification listening on port 8080...")
	log.Fatal(internal.StartServers(service, consumer, 8080))
}


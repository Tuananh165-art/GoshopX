package main

import (
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/inventory/config"
	"github.com/Tuananh165art/GoshopX/inventory/internal"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var repository internal.Repository

	var producer sarama.AsyncProducer
	producer, err := sarama.NewAsyncProducer([]string{config.BootstrapServers}, nil)
	if err != nil {
		log.Println("Kafka producer unavailable:", err)
		producer = nil
	} else {
		defer func() {
			if err := producer.Close(); err != nil {
				log.Println(err)
			}
		}()
	}

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

	service := internal.NewInventoryService(repository, producer)
	log.Println("Inventory listening on port 8080...")
	log.Fatal(internal.ListenGRPC(service, 8080))
}


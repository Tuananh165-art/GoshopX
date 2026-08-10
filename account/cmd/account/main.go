package main

import (
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/account/config"
	"github.com/Tuananh165art/GoshopX/account/internal"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var repository internal.Repository
	var producer sarama.AsyncProducer
	var err error
	if config.BootstrapServers != "" {
		producer, err = sarama.NewAsyncProducer([]string{config.BootstrapServers}, nil)
		if err != nil {
			log.Println("Kafka producer unavailable:", err)
			producer = nil
		}
		if producer != nil {
			defer producer.Close()
		}
	}

	retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
		db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
		if err != nil {
			log.Fatal(err)
		}
		repository, err = internal.NewPostgresRepository(db)
		if err != nil {
			log.Println(err)
		}
		return
	})
	defer repository.Close()
	log.Println("Listening on port 8080...")
	service := internal.NewService(repository, producer)
	log.Fatal(internal.ListenGRPC(service, 8080))
}

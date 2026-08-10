package main

import (
	"context"
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/admin/config"
	"github.com/Tuananh165art/GoshopX/admin/internal"
	"github.com/redis/go-redis/v9"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var repository internal.Repository
	retry.ForeverSleep(2*time.Second, func(_ int) error {
		db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
		if err != nil {
			return err
		}
		repository, err = internal.NewPostgresRepository(db)
		return err
	})
	defer repository.Close()

	if config.RedisURL != "" {
		redisClient := redis.NewClient(&redis.Options{Addr: config.RedisURL})
		defer redisClient.Close()
	}
	service := internal.NewService(repository)
	if config.BootstrapServers != "" {
		if consumer, err := sarama.NewConsumer([]string{config.BootstrapServers}, nil); err == nil {
			defer consumer.Close()
			internal.NewConsumer(consumer, repository).Start(context.Background(), []string{config.ProductTopic, config.OrderTopic, config.PaymentTopic, config.InventoryTopic, config.AdminTopic})
		} else {
			log.Printf("admin kafka consumer unavailable: %v", err)
		}
	}
	log.Fatal(internal.ListenGRPC(service, 8080))
}

package main

import (
	"context"
	"log"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/cart/config"
	"github.com/Tuananh165art/GoshopX/cart/internal"
	inventory "github.com/Tuananh165art/GoshopX/inventory/client"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
)

func main() {
	shutdownTracing, traceErr := observability.ConfigureTracing(context.Background(), "cart")
	if traceErr != nil {
		log.Printf("OpenTelemetry tracing disabled: %v", traceErr)
	} else {
		defer func() { _ = shutdownTracing(context.Background()) }()
	}
	observability.StartMetricsServer(9090)
	repository, err := internal.NewRedisRepository(config.RedisURL)
	if err != nil {
		log.Fatal(err)
	}

	inventoryClient, err := inventory.NewClient(config.InventoryURL)
	if err != nil {
		log.Fatal(err)
	}
	defer inventoryClient.Close()

	var producer sarama.AsyncProducer
	producer, err = sarama.NewAsyncProducer([]string{config.BootstrapServers}, nil)
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

	service := internal.NewCartService(repository, inventoryClient, producer)
	log.Println("Cart listening on port 8080...")
	log.Fatal(internal.ListenGRPC(service, 8080))
}

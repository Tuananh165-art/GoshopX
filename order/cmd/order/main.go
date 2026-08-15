package main

import (
	"context"
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/order/config"
	"github.com/Tuananh165art/GoshopX/order/internal"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	shutdownTracing, traceErr := observability.ConfigureTracing(context.Background(), "order")
	if traceErr != nil {
		log.Printf("OpenTelemetry tracing disabled: %v", traceErr)
	} else {
		defer func() { _ = shutdownTracing(context.Background()) }()
	}
	observability.StartMetricsServer(9090)
	var repository internal.Repository

	producer, err := sarama.NewAsyncProducer([]string{config.BootstrapServers}, nil)
	if err != nil {
		log.Println(err)
	}
	defer func(producer sarama.AsyncProducer) {
		err := producer.Close()
		if err != nil {
			log.Println(err)
		}
	}(producer)

	retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
		db, err := gorm.Open(postgres.Open(config.DatabaseUrl), &gorm.Config{})
		if err != nil {
			log.Println(err)
		}
		repository, err = internal.NewPostgresRepository(db)
		if err != nil {
			log.Println(err)
		}
		return
	})
	defer repository.Close()
	log.Println("Listening on port 8080...")
	service := internal.NewOrderService(repository, producer)
	log.Fatal(internal.ListenGRPC(service, config.AccountUrl, config.ProductUrl, 8080))
}

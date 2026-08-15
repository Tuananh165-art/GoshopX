package main

import (
	"context"
	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/payment/config"
	"github.com/Tuananh165art/GoshopX/payment/internal"
	"github.com/Tuananh165art/GoshopX/payment/vnpay"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	productclient "github.com/Tuananh165art/GoshopX/product/client"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"time"
)

func main() {
	shutdownTracing, traceErr := observability.ConfigureTracing(context.Background(), "payment")
	if traceErr != nil {
		log.Printf("OpenTelemetry tracing disabled: %v", traceErr)
	} else {
		defer func() { _ = shutdownTracing(context.Background()) }()
	}
	observability.StartMetricsServer(9090)
	var repo internal.Repository
	var err error
	retry.ForeverSleep(2*time.Second, func(_ int) error {
		db, e := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
		if e != nil {
			return e
		}
		repo, e = internal.NewPostgresRepository(db)
		return e
	})
	var producer sarama.AsyncProducer
	if config.KafkaBrokers != "" {
		producer, err = sarama.NewAsyncProducer([]string{config.KafkaBrokers}, nil)
		if err != nil {
			log.Printf("Kafka producer unavailable: %v", err)
			producer = nil
		}
	}
	client, err := vnpay.NewClient(vnpay.Config{TmnCode: config.VNPAYTmnCode, HashSecret: config.VNPAYHashSecret, PaymentURL: config.VNPAYPaymentURL})
	if err != nil {
		log.Fatal(err)
	}
	catalog, err := productclient.NewClient(config.ProductServiceURL)
	if err != nil {
		log.Fatal(err)
	}
	defer catalog.Close()
	service := internal.NewVNPAYPaymentServiceWithCatalog(client, repo, producer, catalog)
	log.Fatal(internal.StartServers(service, nil, config.OrderServiceURL, config.InventoryServiceURL, config.CartServiceURL, config.GrpcPort, config.WebhookPort))
}

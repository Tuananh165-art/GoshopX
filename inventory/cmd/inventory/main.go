package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	if config.DummyJSONSeedEnabled {
		if err := seedInventory(context.Background(), service); err != nil {
			log.Printf("Inventory seed unavailable: %v", err)
		} else {
			log.Println("Seeded inventory availability from DummyJSON")
		}
	}
	log.Println("Inventory listening on port 8080...")
	log.Fatal(internal.ListenGRPC(service, 8080))
}

type dummyJSONInventoryResponse struct {
	Products []struct {
		ID    int `json:"id"`
		Stock int `json:"stock"`
	} `json:"products"`
}

func seedInventory(ctx context.Context, service internal.Service) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.DummyJSONURL+"/products?limit=0", nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("dummyjson returned HTTP %d", response.StatusCode)
	}
	var payload dummyJSONInventoryResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return err
	}
	for _, product := range payload.Products {
		if _, _, err := service.UpsertStock(ctx, strconv.Itoa(product.ID), int32(product.Stock), 0); err != nil {
			return err
		}
	}
	return nil
}

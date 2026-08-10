package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/product/config"
	"github.com/tinrab/retry"

	"github.com/Tuananh165art/GoshopX/product/internal"
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
		repository, err = internal.NewElasticRepository(config.DatabaseURL)
		if err != nil {
			log.Println(err)
		}
		return
	})
	defer repository.Close()
	log.Println("Listening on port 8080...")
	service := internal.NewProductService(repository, producer)
	if config.DummyJSONSeedEnabled {
		seedCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		seeded, seedErr := internal.NewDummyJSONClient(config.DummyJSONBaseURL, nil).Seed(seedCtx, repository, config.DummyJSONSeedLimit)
		cancel()
		if seedErr != nil {
			log.Fatalf("DummyJSON seed failed after %d products: %v", seeded, seedErr)
		}
		log.Printf("Seeded %d products from DummyJSON", seeded)
	}

	// Expose a lightweight HTTP read-only endpoint for the recommender sync.
	go func() {
		httpMux := http.NewServeMux()
		httpMux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()
			skipStr := r.URL.Query().Get("skip")
			takeStr := r.URL.Query().Get("take")
			skip, _ := strconv.ParseUint(skipStr, 10, 64)
			take, _ := strconv.ParseUint(takeStr, 10, 64)
			if take == 0 || take > 1000 {
				take = 1000
			}
			products, err := repository.ListAllProducts(ctx, skip, take)
			if err != nil {
				log.Println("HTTP /products error:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(products)
		})
		if err := http.ListenAndServe(":8081", httpMux); err != nil {
			log.Fatalf("HTTP product sync listener failed: %v", err)
		}
	}()

	log.Fatal(internal.ListenGRPC(service, 8080))
}

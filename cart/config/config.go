package config

import "os"

var (
	RedisURL         string
	InventoryURL     string
	BootstrapServers string
	CartTopic        string
	ReservationTTL   int64
)

func init() {
	RedisURL = os.Getenv("REDIS_URL")
	InventoryURL = os.Getenv("INVENTORY_SERVICE_URL")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	CartTopic = os.Getenv("CART_EVENTS_TOPIC")
	if CartTopic == "" {
		CartTopic = "cart_events"
	}
	ReservationTTL = 900
}

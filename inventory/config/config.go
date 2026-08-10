package config

import "os"

var (
	DatabaseURL          string
	BootstrapServers     string
	InventoryTopic       string
	DummyJSONSeedEnabled bool
	DummyJSONURL         string
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	InventoryTopic = os.Getenv("INVENTORY_EVENTS_TOPIC")
	if InventoryTopic == "" {
		InventoryTopic = "inventory_events"
	}
	DummyJSONSeedEnabled = os.Getenv("DUMMYJSON_SEED_ENABLED") == "true"
	DummyJSONURL = os.Getenv("DUMMYJSON_BASE_URL")
	if DummyJSONURL == "" {
		DummyJSONURL = "https://dummyjson.com"
	}
}

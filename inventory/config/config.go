package config

import "os"

var (
	DatabaseURL      string
	BootstrapServers string
	InventoryTopic   string
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	InventoryTopic = os.Getenv("INVENTORY_EVENTS_TOPIC")
	if InventoryTopic == "" {
		InventoryTopic = "inventory_events"
	}
}

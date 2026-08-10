package config

import "os"

var (
	DatabaseURL      = os.Getenv("DATABASE_URL")
	RedisURL         = os.Getenv("REDIS_URL")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	ProductTopic     = valueOrDefault("PRODUCT_EVENTS_TOPIC", "product_events")
	OrderTopic       = valueOrDefault("ORDER_EVENTS_TOPIC", "order_events")
	PaymentTopic     = valueOrDefault("PAYMENT_EVENTS_TOPIC", "payment_events")
	InventoryTopic   = valueOrDefault("INVENTORY_EVENTS_TOPIC", "inventory_events")
	AdminTopic       = valueOrDefault("ADMIN_EVENTS_TOPIC", "admin_events")
)

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

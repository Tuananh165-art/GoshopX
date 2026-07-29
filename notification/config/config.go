package config

import "os"

var (
	DatabaseURL       string
	KafkaBrokers      string
	CartTopic         string
	InventoryTopic    string
	OrderTopic        string
	PaymentTopic      string
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	KafkaBrokers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	CartTopic = getenvDefault("CART_EVENTS_TOPIC", "cart_events")
	InventoryTopic = getenvDefault("INVENTORY_EVENTS_TOPIC", "inventory_events")
	OrderTopic = getenvDefault("ORDER_EVENTS_TOPIC", "order_events")
	PaymentTopic = getenvDefault("PAYMENT_EVENTS_TOPIC", "payment_events")
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

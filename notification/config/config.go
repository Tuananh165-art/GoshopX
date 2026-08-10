package config

import "os"

var (
	DatabaseURL       string
	KafkaBrokers      string
	CartTopic         string
	InventoryTopic    string
	OrderTopic        string
	PaymentTopic      string
	AccountTopic      string
	AccountServiceURL string
	GmailSMTPHost     string
	GmailSMTPPort     string
	GmailUsername     string
	GmailPassword     string
	GmailFrom         string
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	KafkaBrokers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	CartTopic = getenvDefault("CART_EVENTS_TOPIC", "cart_events")
	InventoryTopic = getenvDefault("INVENTORY_EVENTS_TOPIC", "inventory_events")
	OrderTopic = getenvDefault("ORDER_EVENTS_TOPIC", "order_events")
	PaymentTopic = getenvDefault("PAYMENT_EVENTS_TOPIC", "payment_events")
	AccountTopic = getenvDefault("ACCOUNT_EVENTS_TOPIC", "account_events")
	AccountServiceURL = getenvDefault("ACCOUNT_SERVICE_URL", "account:8080")
	GmailSMTPHost = getenvDefault("GMAIL_SMTP_HOST", "smtp.gmail.com")
	GmailSMTPPort = getenvDefault("GMAIL_SMTP_PORT", "587")
	GmailUsername = os.Getenv("GMAIL_USERNAME")
	GmailPassword = os.Getenv("GMAIL_PASSWORD")
	GmailFrom = getenvDefault("GMAIL_FROM", GmailUsername)
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

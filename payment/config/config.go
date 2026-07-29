package config

import "os"

var (
	DatabaseURL        string
	DodoAPIKEY         string
	DodoWebhookSecret  string
	DodoCheckoutURL    string
	DodoTestMode       bool
	OrderServiceURL    string
	InventoryServiceURL string
	CartServiceURL      string
	KafkaBrokers       string
	ProductEventsTopic string
	PaymentEventsTopic string
)

const (
	WebhookPort int = 8081
	GrpcPort    int = 8080
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	OrderServiceURL = os.Getenv("ORDER_SERVICE_URL")
	InventoryServiceURL = os.Getenv("INVENTORY_SERVICE_URL")
	CartServiceURL = os.Getenv("CART_SERVICE_URL")
	DodoAPIKEY = os.Getenv("DODO_API_KEY")
	DodoWebhookSecret = os.Getenv("DODO_WEBHOOK_SECRET")
	DodoCheckoutURL = os.Getenv("DODO_CHECKOUT_URL")
	DodoTestMode = os.Getenv("DODO_TEST_MODE") == "true"
	KafkaBrokers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	ProductEventsTopic = os.Getenv("PRODUCT_EVENTS_TOPIC")
	if ProductEventsTopic == "" {
		ProductEventsTopic = "product_events"
	}
	PaymentEventsTopic = os.Getenv("PAYMENT_EVENTS_TOPIC")
	if PaymentEventsTopic == "" {
		PaymentEventsTopic = "payment_events"
	}
}

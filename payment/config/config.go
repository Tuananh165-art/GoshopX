package config

import (
	"os"
	"strconv"
)

var (
	DatabaseURL         string
	VNPAYTmnCode        string
	VNPAYHashSecret     string
	VNPAYPaymentURL     string
	OrderServiceURL     string
	InventoryServiceURL string
	CartServiceURL      string
	ProductServiceURL   string
	KafkaBrokers        string
	ProductEventsTopic  string
	PaymentEventsTopic  string
	VNDPerUSD           float64
)

const (
	WebhookPort int = 8081
	GrpcPort    int = 8080
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	VNPAYTmnCode = os.Getenv("VNPAY_TMN_CODE")
	VNPAYHashSecret = os.Getenv("VNPAY_HASH_SECRET")
	VNPAYPaymentURL = os.Getenv("VNPAY_PAYMENT_URL")
	OrderServiceURL = os.Getenv("ORDER_SERVICE_URL")
	InventoryServiceURL = os.Getenv("INVENTORY_SERVICE_URL")
	CartServiceURL = os.Getenv("CART_SERVICE_URL")
	ProductServiceURL = os.Getenv("PRODUCT_SERVICE_URL")
	if ProductServiceURL == "" {
		ProductServiceURL = "product:8080"
	}
	KafkaBrokers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	ProductEventsTopic = os.Getenv("PRODUCT_EVENTS_TOPIC")
	if ProductEventsTopic == "" {
		ProductEventsTopic = "product_events"
	}
	PaymentEventsTopic = os.Getenv("PAYMENT_EVENTS_TOPIC")
	if PaymentEventsTopic == "" {
		PaymentEventsTopic = "payment_events"
	}
	VNDPerUSD = 25000
	if value := os.Getenv("VND_PER_USD"); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil && parsed > 0 {
			VNDPerUSD = parsed
		}
	}
}

package config

import "os"

var (
	DatabaseUrl      string
	AccountUrl       string
	ProductUrl       string
	BootstrapServers string
)

func init() {
	DatabaseUrl = os.Getenv("DATABASE_URL")
	AccountUrl = os.Getenv("ACCOUNT_SERVICE_URL")
	ProductUrl = os.Getenv("PRODUCT_SERVICE_URL")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
}

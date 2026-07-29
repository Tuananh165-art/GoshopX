package config

import "os"

var (
	DatabaseURL      string
	BootstrapServers string
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
}

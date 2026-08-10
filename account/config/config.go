package config

import "os"

var (
	DatabaseURL        string
	SecretKey          string
	Issuer             string
	BootstrapServers   string
	AdminEventsTopic   string
	GoogleClientID     string
	AccountEventsTopic string
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	SecretKey = os.Getenv("SECRET_KEY")
	Issuer = os.Getenv("ISSUER")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	AdminEventsTopic = os.Getenv("ADMIN_EVENTS_TOPIC")
	GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	AccountEventsTopic = getenvDefault("ACCOUNT_EVENTS_TOPIC", "account_events")
	if AdminEventsTopic == "" {
		AdminEventsTopic = "admin_events"
	}
}

func getenvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

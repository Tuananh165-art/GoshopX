package config

import (
	"os"
	"strconv"
)

var (
	DatabaseURL          string
	BootstrapServers     string
	DummyJSONBaseURL     string
	DummyJSONSeedEnabled bool
	DummyJSONSeedLimit   uint64
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	BootstrapServers = os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	DummyJSONBaseURL = os.Getenv("DUMMYJSON_BASE_URL")
	if DummyJSONBaseURL == "" {
		DummyJSONBaseURL = "https://dummyjson.com"
	}
	DummyJSONSeedEnabled = os.Getenv("DUMMYJSON_SEED_ENABLED") == "true"
	if value := os.Getenv("DUMMYJSON_SEED_LIMIT"); value != "" {
		if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
			DummyJSONSeedLimit = parsed
		}
	}
}

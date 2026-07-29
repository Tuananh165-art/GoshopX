package config

import "os"

var (
	DatabaseURL string
	SecretKey   string
	Issuer      string
)

func init() {
	DatabaseURL = os.Getenv("DATABASE_URL")
	SecretKey = os.Getenv("SECRET_KEY")
	Issuer = os.Getenv("ISSUER")
}

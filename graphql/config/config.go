package config

import "os"

var (
	AccountUrl     string
	ProductUrl     string
	OrderUrl       string
	PaymentUrl     string
	RecommenderUrl string
	SecretKey      string
	Issuer         string
)

func init() {
	AccountUrl = os.Getenv("ACCOUNT_SERVICE_URL")
	ProductUrl = os.Getenv("PRODUCT_SERVICE_URL")
	OrderUrl = os.Getenv("ORDER_SERVICE_URL")
	PaymentUrl = os.Getenv("PAYMENT_SERVICE_URL")
	RecommenderUrl = os.Getenv("RECOMMENDER_SERVICE_URL")
	SecretKey = os.Getenv("SECRET_KEY")
	Issuer = os.Getenv("ISSUER")
}

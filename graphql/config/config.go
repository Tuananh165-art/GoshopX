package config

import "os"

var (
	AccountUrl      string
	ProductUrl      string
	OrderUrl        string
	PaymentUrl      string
	RecommenderUrl  string
	InventoryUrl    string
	CartUrl         string
	NotificationUrl string
	SecretKey       string
	Issuer          string
)

func init() {
	AccountUrl = os.Getenv("ACCOUNT_SERVICE_URL")
	ProductUrl = os.Getenv("PRODUCT_SERVICE_URL")
	OrderUrl = os.Getenv("ORDER_SERVICE_URL")
	PaymentUrl = os.Getenv("PAYMENT_SERVICE_URL")
	RecommenderUrl = os.Getenv("RECOMMENDER_SERVICE_URL")
	InventoryUrl = os.Getenv("INVENTORY_SERVICE_URL")
	CartUrl = os.Getenv("CART_SERVICE_URL")
	NotificationUrl = os.Getenv("NOTIFICATION_SERVICE_URL")
	SecretKey = os.Getenv("SECRET_KEY")
	Issuer = os.Getenv("ISSUER")
}

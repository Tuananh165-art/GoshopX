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
	AdminUrl        string
	SecretKey       string
	Issuer          string
	MinIOEndpoint   string
	MinIOAccessKey  string
	MinIOSecretKey  string
	MinIOBucket     string
	MinIOUseSSL     string
	MinIOPublicURL  string
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
	AdminUrl = os.Getenv("ADMIN_SERVICE_URL")
	SecretKey = os.Getenv("SECRET_KEY")
	Issuer = os.Getenv("ISSUER")
	MinIOEndpoint = os.Getenv("MINIO_ENDPOINT")
	MinIOAccessKey = os.Getenv("MINIO_ACCESS_KEY")
	MinIOSecretKey = os.Getenv("MINIO_SECRET_KEY")
	MinIOBucket = os.Getenv("MINIO_BUCKET")
	MinIOUseSSL = os.Getenv("MINIO_USE_SSL")
	MinIOPublicURL = os.Getenv("MINIO_PUBLIC_URL")
}

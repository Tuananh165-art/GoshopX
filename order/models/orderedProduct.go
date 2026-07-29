package models

type ProductsInfo struct {
	ID        uint `gorm:"primaryKey;autoIncrement"`
	OrderID   uint
	ProductID string
	Quantity  int
}

func (ProductsInfo) TableName() string {
	return "order_products"
}

package models

type ProductsInfo struct {
	ID          uint `gorm:"primaryKey;autoIncrement"`
	OrderID     uint
	ProductID   string
	Quantity    int
	Name        string
	Description string `gorm:"type:text"`
	Price       float64
}

func (ProductsInfo) TableName() string {
	return "order_products"
}

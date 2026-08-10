package models

import "time"

type Order struct {
	ID            uint `gorm:"primaryKey;autoIncrement"`
	CreatedAt     time.Time
	TotalPrice    float64
	AccountID     uint64
	Status        string
	PaymentStatus string
	ProductsInfos []ProductsInfo    `gorm:"foreignKey:OrderID"`
	Products      []*OrderedProduct `gorm:"-"`
}

type OrderedProduct struct {
	ID          string
	Name        string
	Description string
	Price       float64
	Quantity    uint32
}

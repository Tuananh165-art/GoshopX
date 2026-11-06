package models

import "time"

type Order struct {
	ID              uint `gorm:"primaryKey;autoIncrement"`
	CreatedAt       time.Time
	TotalPrice      float64
	SubtotalVND     int64
	DiscountVND     int64
	TotalPriceVND   int64
	CouponCode      string  `gorm:"size:64"`
	CheckoutQuoteID *string `gorm:"size:64;uniqueIndex"`
	AccountID       uint64
	Status          string
	PaymentStatus   string
	ProductsInfos   []ProductsInfo    `gorm:"foreignKey:OrderID"`
	Products        []*OrderedProduct `gorm:"-"`
}

type OrderedProduct struct {
	ID          string
	Name        string
	Description string
	Price       float64
	Quantity    uint32
}

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Coupon is owned by Order so external model output cannot grant a discount.
type Coupon struct {
	Code               string `gorm:"primaryKey;size:64"`
	PercentBasisPoint  int64
	FixedAmountVND     int64
	MinimumSubtotalVND int64
	MaximumDiscountVND int64
	MaximumRedemptions int64
	PerAccountLimit    int64
	StartsAt           time.Time
	ExpiresAt          time.Time
	Active             bool
}

type CouponRedemption struct {
	ID         uint   `gorm:"primaryKey"`
	CouponCode string `gorm:"size:64;index;uniqueIndex:idx_coupon_order"`
	OrderID    uint64 `gorm:"uniqueIndex:idx_coupon_order"`
	AccountID  uint64 `gorm:"index"`
	CreatedAt  time.Time
}

type CheckoutQuote struct {
	ID            string
	SubtotalVND   int64
	DiscountVND   int64
	TotalPriceVND int64
	CouponCode    string
	ExpiresAt     time.Time
	CartVersion   uint64
}

type CheckoutQuoteRecord struct {
	ID            string `gorm:"primaryKey;size:64"`
	AccountID     uint64 `gorm:"index"`
	ItemsHash     string `gorm:"size:64"`
	SubtotalVND   int64
	DiscountVND   int64
	TotalPriceVND int64
	CouponCode    string `gorm:"size:64"`
	ExpiresAt     time.Time
	CartVersion   uint64
	OrderID       uint64
	CreatedAt     time.Time
}

func CheckoutItemsHash(products []*OrderedProduct) string {
	items := make([]*OrderedProduct, 0, len(products))
	for _, product := range products {
		if product != nil {
			items = append(items, product)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	h := sha256.New()
	for _, item := range items {
		if item == nil {
			continue
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\n", item.ID, item.Quantity)
	}
	return hex.EncodeToString(h.Sum(nil))
}

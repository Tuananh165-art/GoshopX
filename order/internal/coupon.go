package internal

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Tuananh165art/GoshopX/order/models"
)

var ErrInvalidCoupon = errors.New("coupon is invalid or unavailable")
var ErrInvalidCheckoutQuote = errors.New("checkout quote is invalid, expired, or no longer matches the cart")

func discountedUnitPriceVND(price, discountPercentage float64) (int64, error) {
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) || discountPercentage < 0 || discountPercentage >= 100 || math.IsNaN(discountPercentage) || math.IsInf(discountPercentage, 0) {
		return 0, ErrInvalidOrderTotal
	}
	discounted := math.Round(price * (1 - discountPercentage/100))
	if discounted <= 0 || discounted >= float64(math.MaxInt64) {
		return 0, ErrInvalidOrderTotal
	}
	return int64(discounted), nil
}

// CouponDiscount computes a deterministic integer-VND discount. Usage limits
// are supplied by the repository while holding its transaction lock.
func CouponDiscount(coupon models.Coupon, subtotalVND, totalUses, accountUses int64, now time.Time) (int64, error) {
	if !coupon.Active || strings.TrimSpace(coupon.Code) == "" || subtotalVND < coupon.MinimumSubtotalVND || subtotalVND < 0 ||
		(!coupon.StartsAt.IsZero() && now.Before(coupon.StartsAt)) || (!coupon.ExpiresAt.IsZero() && !now.Before(coupon.ExpiresAt)) ||
		(coupon.MaximumRedemptions > 0 && totalUses >= coupon.MaximumRedemptions) ||
		(coupon.PerAccountLimit > 0 && accountUses >= coupon.PerAccountLimit) {
		return 0, ErrInvalidCoupon
	}
	if (coupon.PercentBasisPoint > 0) == (coupon.FixedAmountVND > 0) || coupon.PercentBasisPoint < 0 || coupon.PercentBasisPoint > 10000 || coupon.FixedAmountVND < 0 {
		return 0, ErrInvalidCoupon
	}
	discount := coupon.FixedAmountVND
	if coupon.PercentBasisPoint > 0 {
		whole, remainder := subtotalVND/10000, subtotalVND%10000
		fraction := remainder * coupon.PercentBasisPoint / 10000
		if whole > (math.MaxInt64-fraction)/coupon.PercentBasisPoint {
			return 0, ErrInvalidCoupon
		}
		discount = whole*coupon.PercentBasisPoint + fraction
	}
	if coupon.MaximumDiscountVND > 0 && discount > coupon.MaximumDiscountVND {
		discount = coupon.MaximumDiscountVND
	}
	if discount > subtotalVND {
		discount = subtotalVND
	}
	return discount, nil
}

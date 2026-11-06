package internal

import (
	"math"
	"testing"
	"time"

	"github.com/Tuananh165art/GoshopX/order/models"
)

func TestCouponDiscount(t *testing.T) {
	now := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	coupon := models.Coupon{Code: "LAB10", PercentBasisPoint: 1000, MinimumSubtotalVND: 1_000_000, MaximumDiscountVND: 100_000, MaximumRedemptions: 50, PerAccountLimit: 1, ExpiresAt: now.Add(time.Hour), Active: true}
	tests := []struct {
		name      string
		subtotal  int64
		uses      int64
		account   int64
		want      int64
		wantError bool
	}{
		{name: "percentage capped", subtotal: 2_000_000, want: 100_000},
		{name: "integer VND floor", subtotal: 1_000_009, want: 100_000},
		{name: "minimum enforced", subtotal: 999_999, wantError: true},
		{name: "global limit enforced", subtotal: 1_000_000, uses: 50, wantError: true},
		{name: "account limit enforced", subtotal: 1_000_000, account: 1, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CouponDiscount(coupon, tt.subtotal, tt.uses, tt.account, now)
			if (err != nil) != tt.wantError || got != tt.want {
				t.Fatalf("CouponDiscount() = (%d, %v), want (%d, error=%t)", got, err, tt.want, tt.wantError)
			}
		})
	}
}

func TestDiscountedUnitPriceVND(t *testing.T) {
	got, err := discountedUnitPriceVND(34_999_750, 9.38)
	if err != nil || got != 31_716_773 {
		t.Fatalf("discountedUnitPriceVND() = %d, %v; want 31716773, nil", got, err)
	}
	for _, tc := range []struct{ price, discount float64 }{{0, 0}, {-1, 0}, {1_000, 100}, {1_000, math.NaN()}, {math.Inf(1), 0}} {
		if _, err := discountedUnitPriceVND(tc.price, tc.discount); err == nil {
			t.Fatalf("discountedUnitPriceVND(%v, %v) should reject invalid input", tc.price, tc.discount)
		}
	}
}

func TestCouponDiscountRejectsExpiredAndAmbiguousRules(t *testing.T) {
	now := time.Now().UTC()
	for _, coupon := range []models.Coupon{
		{Code: "OLD", PercentBasisPoint: 1000, ExpiresAt: now, Active: true},
		{Code: "AMBIGUOUS", PercentBasisPoint: 1000, FixedAmountVND: 1, Active: true},
	} {
		if _, err := CouponDiscount(coupon, 10_000, 0, 0, now); err == nil {
			t.Fatalf("expected coupon %q to be rejected", coupon.Code)
		}
	}
}

package tests

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/Tuananh165art/GoshopX/order/internal"
	"github.com/Tuananh165art/GoshopX/order/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupBenchmarkDB(b *testing.B) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(b, err)

	err = db.AutoMigrate(&models.Order{}, &models.ProductsInfo{})
	require.NoError(b, err)

	return db
}

func setupBenchmarkRepository(b *testing.B) internal.Repository {
	db := setupBenchmarkDB(b)
	r, err := internal.NewPostgresRepository(db)
	if err != nil {
		b.Fatal(err)
	}
	return r
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Order{}, &models.ProductsInfo{})
	require.NoError(t, err)

	return db
}

func setupTestRepository(t *testing.T) internal.Repository {
	db := setupTestDB(t)
	r, err := internal.NewPostgresRepository(db)
	if err != nil {
		log.Println(err)
	}
	return r
}

func createSampleOrder(accountID uint64) *models.Order {
	return &models.Order{
		AccountID:  accountID,
		TotalPrice: 99.99,
		CreatedAt:  time.Now().UTC(),
		Products: []*models.OrderedProduct{
			{
				ID:       "product-1",
				Name:     "Product One",
				Price:    49.99,
				Quantity: 1,
			},
			{
				ID:       "product-2",
				Name:     "Product Two",
				Price:    50.00,
				Quantity: 2,
			},
		},
	}
}

func TestRepository_PutOrder(t *testing.T) {
	db := setupTestDB(t)
	repo, err := internal.NewPostgresRepository(db)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	order := createSampleOrder(1)

	t.Run("successful order creation", func(t *testing.T) {
		err := repo.PutOrder(ctx, order)

		assert.NoError(t, err)
		assert.NotZero(t, order.ID)

		var savedOrder models.Order
		err = db.First(&savedOrder, order.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, uint64(1), savedOrder.AccountID)
		assert.InDelta(t, 99.99, savedOrder.TotalPrice, 0.001)

		var productInfos []models.ProductsInfo
		err = db.Where("order_id = ?", order.ID).Find(&productInfos).Error
		assert.NoError(t, err)
		assert.Len(t, productInfos, 2)
		assert.Equal(t, "product-1", productInfos[0].ProductID)
		assert.Equal(t, 1, productInfos[0].Quantity)
		assert.Equal(t, "product-2", productInfos[1].ProductID)
		assert.Equal(t, 2, productInfos[1].Quantity)
	})

	t.Run("order with no products", func(t *testing.T) {
		emptyOrder := &models.Order{
			AccountID:  2,
			TotalPrice: 10.00,
			CreatedAt:  time.Now().UTC(),
		}

		err := repo.PutOrder(ctx, emptyOrder)

		assert.NoError(t, err)
		assert.NotZero(t, emptyOrder.ID)

		var productInfos []models.ProductsInfo
		err = db.Where("order_id = ?", emptyOrder.ID).Find(&productInfos).Error
		assert.NoError(t, err)
		assert.Len(t, productInfos, 0)
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		newOrder := createSampleOrder(3)

		err := repo.PutOrder(ctx, newOrder)
		if err != nil {
			t.Logf("Expected behavior: context cancellation handled: %v", err)
		}
	})
}

func TestRepository_PutOrderWithCouponUsesAndRecordsCouponAtomically(t *testing.T) {
	db := setupTestDB(t)
	repo, err := internal.NewPostgresRepository(db)
	require.NoError(t, err)
	coupon := models.Coupon{Code: "LAB10", PercentBasisPoint: 1000, MinimumSubtotalVND: 1_000_000, MaximumDiscountVND: 100_000, MaximumRedemptions: 2, PerAccountLimit: 1, ExpiresAt: time.Now().Add(time.Hour), Active: true}
	require.NoError(t, db.Create(&coupon).Error)

	order := &models.Order{AccountID: 42, SubtotalVND: 2_000_000, TotalPriceVND: 2_000_000, TotalPrice: 2_000_000, CreatedAt: time.Now().UTC(), Products: []*models.OrderedProduct{{ID: "p1", Name: "Test", Price: 2_000_000, Quantity: 1}}}
	require.NoError(t, repo.PutOrderWithCoupon(context.Background(), order, " lab10 "))
	assert.Equal(t, "LAB10", order.CouponCode)
	assert.Equal(t, int64(100_000), order.DiscountVND)
	assert.Equal(t, int64(1_900_000), order.TotalPriceVND)

	var saved models.Order
	require.NoError(t, db.First(&saved, order.ID).Error)
	assert.Equal(t, int64(1_900_000), saved.TotalPriceVND)
	var uses int64
	require.NoError(t, db.Model(&models.CouponRedemption{}).Where("coupon_code = ?", "LAB10").Count(&uses).Error)
	assert.Equal(t, int64(1), uses)
	discount, err := repo.PreviewCoupon(context.Background(), 7, 2_000_000, "lab10")
	require.NoError(t, err)
	assert.Equal(t, int64(100_000), discount)
	require.NoError(t, db.Model(&models.CouponRedemption{}).Where("coupon_code = ?", "LAB10").Count(&uses).Error)
	assert.Equal(t, int64(1), uses, "preview must not consume a redemption")

	second := &models.Order{AccountID: 42, SubtotalVND: 2_000_000, TotalPrice: 2_000_000, CreatedAt: time.Now().UTC()}
	err = repo.PutOrderWithCoupon(context.Background(), second, "LAB10")
	assert.ErrorIs(t, err, internal.ErrInvalidCoupon)
	assert.Zero(t, second.ID)
}

func TestRepository_PutOrderWithQuoteValidatesAndConsumesIdempotently(t *testing.T) {
	db := setupTestDB(t)
	repo, err := internal.NewPostgresRepository(db)
	require.NoError(t, err)
	defer repo.Close()
	products := []*models.OrderedProduct{{ID: "p1", Name: "Phone", Price: 2_000_000, Quantity: 1}}
	require.NoError(t, db.Create(&models.Coupon{Code: "LAB10", PercentBasisPoint: 1000, MinimumSubtotalVND: 1_000_000, MaximumDiscountVND: 100_000, MaximumRedemptions: 1, PerAccountLimit: 1, ExpiresAt: time.Now().UTC().Add(time.Hour), Active: true}).Error)
	quote := &models.CheckoutQuoteRecord{ID: "quote-1", AccountID: 42, ItemsHash: models.CheckoutItemsHash(products), SubtotalVND: 2_000_000, DiscountVND: 100_000, TotalPriceVND: 1_900_000, CouponCode: "LAB10", ExpiresAt: time.Now().UTC().Add(time.Minute), CartVersion: 9}
	require.NoError(t, repo.CreateCheckoutQuote(context.Background(), quote))
	order := &models.Order{AccountID: 42, SubtotalVND: 2_000_000, TotalPriceVND: 2_000_000, TotalPrice: 2_000_000, CreatedAt: time.Now().UTC(), Products: products}
	created, err := repo.PutOrderWithQuote(context.Background(), order, quote.ID, quote.ItemsHash, quote.CartVersion)
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotZero(t, order.ID)
	assert.Equal(t, int64(100_000), order.DiscountVND)
	assert.Equal(t, int64(1_900_000), order.TotalPriceVND)

	retry := &models.Order{AccountID: 42}
	created, err = repo.PutOrderWithQuote(context.Background(), retry, quote.ID, quote.ItemsHash, quote.CartVersion)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, order.ID, retry.ID)
	assert.Len(t, retry.Products, 1)
	require.NotNil(t, retry.CheckoutQuoteID)
	assert.Equal(t, quote.ID, *retry.CheckoutQuoteID)
	var uses int64
	require.NoError(t, db.Model(&models.CouponRedemption{}).Where("coupon_code = ?", "LAB10").Count(&uses).Error)
	assert.Equal(t, int64(1), uses, "retrying an already-consumed quote must not redeem the coupon twice")
}

func TestRepository_PutOrderWithQuoteRejectsWrongOwnerStaleVersionAndExpiry(t *testing.T) {
	for _, tc := range []struct {
		name    string
		owner   uint64
		version uint64
		expired bool
	}{
		{name: "wrong owner", owner: 7, version: 9},
		{name: "stale cart", owner: 42, version: 8},
		{name: "expired", owner: 42, version: 9, expired: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTestDB(t)
			repo, err := internal.NewPostgresRepository(db)
			require.NoError(t, err)
			defer repo.Close()
			products := []*models.OrderedProduct{{ID: "p1", Quantity: 1}}
			expires := time.Now().UTC().Add(time.Minute)
			if tc.expired {
				expires = time.Now().UTC().Add(-time.Minute)
			}
			quote := &models.CheckoutQuoteRecord{ID: "quote-1", AccountID: 42, ItemsHash: models.CheckoutItemsHash(products), SubtotalVND: 500, ExpiresAt: expires, CartVersion: 9}
			require.NoError(t, repo.CreateCheckoutQuote(context.Background(), quote))
			order := &models.Order{AccountID: tc.owner, SubtotalVND: 500, Products: products}
			_, err = repo.PutOrderWithQuote(context.Background(), order, quote.ID, quote.ItemsHash, tc.version)
			assert.ErrorIs(t, err, internal.ErrInvalidCheckoutQuote)
		})
	}
}

func TestRepository_GetOrdersForAccount(t *testing.T) {
	t.Skip("Skipping test - GetOrdersForAccount uses PostgreSQL-specific SQL")
}

func TestRepository_UpdateOrderPaymentStatus(t *testing.T) {
	db := setupTestDB(t)
	repo, err := internal.NewPostgresRepository(db)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	order := createSampleOrder(1)

	err = repo.PutOrder(ctx, order)
	require.NoError(t, err)

	t.Run("successful payment status update", func(t *testing.T) {
		err := repo.UpdateOrderPaymentStatus(ctx, uint64(order.ID), "paid")

		assert.NoError(t, err)

		var savedOrder models.Order
		err = db.First(&savedOrder, order.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, "paid", savedOrder.PaymentStatus)
	})

	t.Run("update non-existent order", func(t *testing.T) {
		err := repo.UpdateOrderPaymentStatus(ctx, 99999, "paid")

		assert.NoError(t, err)
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := repo.UpdateOrderPaymentStatus(ctx, uint64(order.ID), "pending")
		if err != nil {
			t.Logf("Context cancellation handled: %v", err)
		}
	})
}

func TestRepository_Close(t *testing.T) {
	repo := setupTestRepository(t)

	assert.NotPanics(t, func() {
		repo.Close()
	})

	ctx := context.Background()
	order := createSampleOrder(1)

	err := repo.PutOrder(ctx, order)
	assert.Error(t, err)
}

func BenchmarkPostgresRepository_PutOrder(b *testing.B) {
	repo := setupBenchmarkRepository(b)
	defer repo.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order := &models.Order{
			AccountID:  uint64(i + 1),
			TotalPrice: float64(i) * 10.5,
			CreatedAt:  time.Now().UTC(),
			Products: []*models.OrderedProduct{
				{
					ID:       fmt.Sprintf("product-%d", i),
					Quantity: 1,
				},
			},
		}

		err := repo.PutOrder(ctx, order)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestWithTimeout(t *testing.T) {
	repo := setupTestRepository(t)
	defer repo.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	order := createSampleOrder(1)

	err := repo.PutOrder(ctx, order)
	if err != nil {
		t.Logf("Operation timed out as expected: %v", err)
	}
}

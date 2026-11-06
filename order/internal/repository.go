package internal

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Tuananh165art/GoshopX/order/models"
	"github.com/Tuananh165art/GoshopX/pkg/migrations"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Close()
	PutOrder(ctx context.Context, order *models.Order) error
	PutOrderWithCoupon(ctx context.Context, order *models.Order, couponCode string) error
	CreateCheckoutQuote(ctx context.Context, quote *models.CheckoutQuoteRecord) error
	PutOrderWithQuote(ctx context.Context, order *models.Order, quoteID, itemsHash string, cartVersion uint64) (bool, error)
	PreviewCoupon(ctx context.Context, accountID uint64, subtotalVND int64, couponCode string) (int64, error)
	GetOrdersForAccount(ctx context.Context, accountId uint64) ([]*models.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, status string) error
	ListOrders(ctx context.Context, status, paymentStatus string, accountID uint64, skip, take uint64) ([]*models.Order, error)
	GetOrderByID(ctx context.Context, orderID uint64) (*models.Order, error)
	CancelOrder(ctx context.Context, orderID uint64, reason string) (*models.Order, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) (Repository, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	err = sqlDB.Ping()
	if err != nil {
		return nil, err
	}

	err = migrations.Run(db, "order", "001_initial", func(db *gorm.DB) error {
		return db.AutoMigrate(&models.Order{}, &models.ProductsInfo{})
	})
	if err != nil {
		return nil, err
	}
	if err = migrations.Run(db, "order", "002_coupon", func(db *gorm.DB) error {
		return db.AutoMigrate(&models.Order{}, &models.Coupon{}, &models.CouponRedemption{})
	}); err != nil {
		return nil, err
	}
	if err = migrations.Run(db, "order", "003_checkout_quote", func(db *gorm.DB) error {
		return db.AutoMigrate(&models.Order{}, &models.CheckoutQuoteRecord{})
	}); err != nil {
		return nil, err
	}

	return &postgresRepository{db}, nil
}

func (repository *postgresRepository) CreateCheckoutQuote(ctx context.Context, quote *models.CheckoutQuoteRecord) error {
	db := repository.db.WithContext(ctx)
	if err := db.Where("expires_at < ?", time.Now().UTC().Add(-7*24*time.Hour)).Delete(&models.CheckoutQuoteRecord{}).Error; err != nil {
		return err
	}
	return db.Create(quote).Error
}

// PutOrderWithQuote consumes the preview and creates the order in one transaction.
func (repository *postgresRepository) PutOrderWithQuote(ctx context.Context, order *models.Order, quoteID, itemsHash string, cartVersion uint64) (bool, error) {
	tx := repository.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return false, tx.Error
	}
	defer tx.Rollback()
	var quote models.CheckoutQuoteRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&quote, "id = ?", quoteID).Error; err != nil {
		return false, ErrInvalidCheckoutQuote
	}
	if quote.AccountID != order.AccountID {
		return false, ErrInvalidCheckoutQuote
	}
	if quote.OrderID != 0 {
		if err := tx.Preload("ProductsInfos").First(order, quote.OrderID).Error; err != nil {
			return false, err
		}
		for _, item := range order.ProductsInfos {
			order.Products = append(order.Products, &models.OrderedProduct{ID: item.ProductID, Name: item.Name, Description: item.Description, Price: item.Price, Quantity: uint32(item.Quantity)})
		}
		if err := tx.Commit().Error; err != nil {
			return false, err
		}
		return false, nil
	}
	now := time.Now().UTC()
	if !now.Before(quote.ExpiresAt) || quote.CartVersion != cartVersion || quote.ItemsHash != itemsHash || quote.SubtotalVND != order.SubtotalVND {
		return false, ErrInvalidCheckoutQuote
	}
	if quote.CouponCode != "" {
		var coupon models.Coupon
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", quote.CouponCode).First(&coupon).Error; err != nil {
			return false, ErrInvalidCoupon
		}
		var totalUses, accountUses int64
		if err := tx.Model(&models.CouponRedemption{}).Where("coupon_code = ?", quote.CouponCode).Count(&totalUses).Error; err != nil {
			return false, err
		}
		if err := tx.Model(&models.CouponRedemption{}).Where("coupon_code = ? AND account_id = ?", quote.CouponCode, order.AccountID).Count(&accountUses).Error; err != nil {
			return false, err
		}
		discount, err := CouponDiscount(coupon, order.SubtotalVND, totalUses, accountUses, now)
		if err != nil {
			return false, err
		}
		order.DiscountVND, order.TotalPriceVND = discount, order.SubtotalVND-discount
		order.TotalPrice, order.CouponCode = float64(order.TotalPriceVND), quote.CouponCode
	}
	order.CheckoutQuoteID = &quote.ID
	if err := tx.Create(order).Error; err != nil {
		return false, err
	}
	for _, product := range order.Products {
		item := models.ProductsInfo{OrderID: order.ID, ProductID: product.ID, Quantity: int(product.Quantity), Name: product.Name, Description: product.Description, Price: product.Price}
		if err := tx.Create(&item).Error; err != nil {
			return false, err
		}
	}
	if quote.CouponCode != "" {
		if err := tx.Create(&models.CouponRedemption{CouponCode: quote.CouponCode, OrderID: uint64(order.ID), AccountID: order.AccountID}).Error; err != nil {
			return false, err
		}
	}
	if err := tx.Model(&quote).Update("order_id", uint64(order.ID)).Error; err != nil {
		return false, err
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return true, nil
}

func (repository *postgresRepository) Close() {
	sqlDB, err := repository.db.DB()
	if err == nil {
		err = sqlDB.Close()
		if err != nil {
			log.Println("Error closing postgres repository")
			log.Println(err)
		}
	}
}

func (repository *postgresRepository) PutOrder(ctx context.Context, order *models.Order) error {
	tx := repository.db.WithContext(ctx).Begin()

	err := tx.WithContext(ctx).Create(&order).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, product := range order.Products {
		orderedProduct := models.ProductsInfo{
			OrderID:     order.ID,
			ProductID:   product.ID,
			Quantity:    int(product.Quantity),
			Name:        product.Name,
			Description: product.Description,
			Price:       product.Price,
		}
		err = tx.Create(&orderedProduct).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

func (repository *postgresRepository) PutOrderWithCoupon(ctx context.Context, order *models.Order, couponCode string) error {
	code := strings.ToUpper(strings.TrimSpace(couponCode))
	if code == "" {
		return repository.PutOrder(ctx, order)
	}
	tx := repository.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	var coupon models.Coupon
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&coupon).Error; err != nil {
		return ErrInvalidCoupon
	}
	var totalUses, accountUses int64
	if err := tx.Model(&models.CouponRedemption{}).Where("coupon_code = ?", code).Count(&totalUses).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.CouponRedemption{}).Where("coupon_code = ? AND account_id = ?", code, order.AccountID).Count(&accountUses).Error; err != nil {
		return err
	}
	discount, err := CouponDiscount(coupon, order.SubtotalVND, totalUses, accountUses, time.Now().UTC())
	if err != nil {
		return err
	}
	order.CouponCode = code
	order.DiscountVND = discount
	order.TotalPriceVND = order.SubtotalVND - discount
	order.TotalPrice = float64(order.TotalPriceVND)
	if err := tx.Create(order).Error; err != nil {
		return err
	}
	for _, product := range order.Products {
		item := models.ProductsInfo{OrderID: order.ID, ProductID: product.ID, Quantity: int(product.Quantity), Name: product.Name, Description: product.Description, Price: product.Price}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	if err := tx.Create(&models.CouponRedemption{CouponCode: code, OrderID: uint64(order.ID), AccountID: order.AccountID}).Error; err != nil {
		return err
	}
	return tx.Commit().Error
}

func (repository *postgresRepository) PreviewCoupon(ctx context.Context, accountID uint64, subtotalVND int64, couponCode string) (int64, error) {
	code := strings.ToUpper(strings.TrimSpace(couponCode))
	if subtotalVND < 0 {
		return 0, ErrInvalidOrderTotal
	}
	if code == "" {
		return 0, nil
	}
	var coupon models.Coupon
	if err := repository.db.WithContext(ctx).Where("code = ?", code).First(&coupon).Error; err != nil {
		return 0, ErrInvalidCoupon
	}
	var totalUses, accountUses int64
	if err := repository.db.WithContext(ctx).Model(&models.CouponRedemption{}).Where("coupon_code = ?", code).Count(&totalUses).Error; err != nil {
		return 0, err
	}
	if err := repository.db.WithContext(ctx).Model(&models.CouponRedemption{}).Where("coupon_code = ? AND account_id = ?", code, accountID).Count(&accountUses).Error; err != nil {
		return 0, err
	}
	return CouponDiscount(coupon, subtotalVND, totalUses, accountUses, time.Now().UTC())
}

func (repository *postgresRepository) GetOrdersForAccount(ctx context.Context, accountId uint64) ([]*models.Order, error) {
	return repository.ListOrders(ctx, "", "", accountId, 0, 0)
}

func (repository *postgresRepository) UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, status string) error {
	return repository.db.WithContext(ctx).Model(&models.Order{}).
		Where("id = ?", orderId).
		Update("payment_status", status).Error
}

func (repository *postgresRepository) ListOrders(ctx context.Context, status, paymentStatus string, accountID uint64, skip, take uint64) ([]*models.Order, error) {
	query := repository.db.WithContext(ctx).Preload("ProductsInfos").Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if paymentStatus != "" {
		query = query.Where("payment_status = ?", paymentStatus)
	}
	if accountID > 0 {
		query = query.Where("account_id = ?", accountID)
	}
	if take > 0 {
		query = query.Offset(int(skip)).Limit(int(take))
	}
	var orders []*models.Order
	if err := query.Find(&orders).Error; err != nil {
		return nil, err
	}
	for _, order := range orders {
		for _, item := range order.ProductsInfos {
			order.Products = append(order.Products, &models.OrderedProduct{ID: item.ProductID, Quantity: uint32(item.Quantity), Name: item.Name, Description: item.Description, Price: item.Price})
		}
	}
	return orders, nil
}

func (repository *postgresRepository) GetOrderByID(ctx context.Context, orderID uint64) (*models.Order, error) {
	orders, err := repository.ListOrders(ctx, "", "", 0, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, order := range orders {
		if uint64(order.ID) == orderID {
			return order, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (repository *postgresRepository) CancelOrder(ctx context.Context, orderID uint64, reason string) (*models.Order, error) {
	order, err := repository.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != "pending" && order.Status != "processing" {
		return nil, gorm.ErrInvalidData
	}
	if err := repository.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", orderID).Updates(map[string]any{"status": "cancelled"}).Error; err != nil {
		return nil, err
	}
	order.Status = "cancelled"
	return order, nil
}

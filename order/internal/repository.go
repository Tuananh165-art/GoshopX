package internal

import (
	"context"
	"log"

	"github.com/Tuananh165art/GoshopX/order/models"
	"github.com/Tuananh165art/GoshopX/pkg/migrations"
	"gorm.io/gorm"
)

type Repository interface {
	Close()
	PutOrder(ctx context.Context, order *models.Order) error
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

	return &postgresRepository{db}, nil
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

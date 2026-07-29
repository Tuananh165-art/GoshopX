package internal

import (
	"context"
	"log"

	"github.com/rasadov/EcommerceAPI/payment/models"
	"gorm.io/gorm"
)

type Repository interface {
	Close()

	GetCustomerByCustomerID(ctx context.Context, customerId string) (*models.Customer, error)
	GetCustomerByUserID(ctx context.Context, userId uint64) (*models.Customer, error)
	SaveCustomer(ctx context.Context, customer *models.Customer) error

	GetProductByProductID(ctx context.Context, productId string) (*models.Product, error)
	GetProductsByIDs(ctx context.Context, productIds []string) ([]*models.Product, error)
	SaveProduct(ctx context.Context, product *models.Product) error
	UpdateProduct(ctx context.Context, product *models.Product) error
	DeleteProduct(ctx context.Context, productId string) error

	RegisterTransaction(ctx context.Context, transaction *models.Transaction) error
	UpdateTransaction(ctx context.Context, transaction *models.Transaction) error
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) (Repository, error) {
	err := db.AutoMigrate(&models.Customer{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&models.Product{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&models.Transaction{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	err = sqlDB.Ping()
	if err != nil {
		return nil, err
	}

	return &postgresRepository{db: db}, nil
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

func (repository *postgresRepository) GetCustomerByCustomerID(ctx context.Context, customerId string) (*models.Customer, error) {
	var customer models.Customer
	err := repository.db.WithContext(ctx).First(&customer, "id = ?", customerId).Error
	if err != nil {
		return nil, err
	}
	return &customer, nil
}

func (repository *postgresRepository) GetCustomerByUserID(ctx context.Context, userId uint64) (*models.Customer, error) {
	var customer models.Customer
	err := repository.db.WithContext(ctx).First(&customer, "user_id = ?", userId).Error
	if err != nil {
		return nil, err
	}
	return &customer, nil
}

func (repository *postgresRepository) GetProductByProductID(ctx context.Context, productId string) (*models.Product, error) {
	var product models.Product
	err := repository.db.WithContext(ctx).First(&product, "product_id = ?", productId).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (repository *postgresRepository) GetProductsByIDs(ctx context.Context, productIds []string) ([]*models.Product, error) {
	var products []*models.Product
	err := repository.db.WithContext(ctx).Find(&products, "product_id IN (?)", productIds).Error
	if err != nil {
		return nil, err
	}
	return products, nil
}

func (repository *postgresRepository) SaveCustomer(ctx context.Context, customer *models.Customer) error {
	return repository.db.WithContext(ctx).Create(&customer).Error
}

func (repository *postgresRepository) SaveProduct(ctx context.Context, product *models.Product) error {
	return repository.db.WithContext(ctx).Create(&product).Error
}

func (repository *postgresRepository) UpdateProduct(ctx context.Context, product *models.Product) error {
	return repository.db.WithContext(ctx).Save(&product).Error
}

func (repository *postgresRepository) DeleteProduct(ctx context.Context, productId string) error {
	return repository.db.WithContext(ctx).Delete(&models.Product{}, "product_id = ?", productId).Error
}

func (repository *postgresRepository) RegisterTransaction(ctx context.Context, transaction *models.Transaction) error {
	return repository.db.WithContext(ctx).Create(&transaction).Error
}

func (repository *postgresRepository) UpdateTransaction(ctx context.Context, transaction *models.Transaction) error {
	return repository.db.WithContext(ctx).Save(&transaction).Error
}

package internal

import (
	"context"
	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/pkg/migrations"
	"gorm.io/gorm"
	"log"
)

type Repository interface {
	Close()
	GetCustomerByCustomerID(context.Context, string) (*models.Customer, error)
	GetCustomerByUserID(context.Context, uint64) (*models.Customer, error)
	SaveCustomer(context.Context, *models.Customer) error
	GetProductByProductID(context.Context, string) (*models.Product, error)
	GetProductsByIDs(context.Context, []string) ([]*models.Product, error)
	SaveProduct(context.Context, *models.Product) error
	UpdateProduct(context.Context, *models.Product) error
	DeleteProduct(context.Context, string) error
	RegisterTransaction(context.Context, *models.Transaction) error
	UpdateTransaction(context.Context, *models.Transaction) error
	ListTransactions(context.Context, string, uint64, uint64, uint64) ([]*models.Transaction, error)
	GetTransactionByPaymentID(context.Context, string) (*models.Transaction, error)
	SaveRefund(context.Context, *models.Refund) error
	GetRefundByKey(context.Context, string) (*models.Refund, error)
}
type postgresRepository struct{ db *gorm.DB }

func NewPostgresRepository(db *gorm.DB) (Repository, error) {
	if e := migrations.Run(db, "payment", "001_initial", func(db *gorm.DB) error {
		for _, m := range []any{&models.Customer{}, &models.Product{}, &models.Transaction{}, &models.Refund{}} {
			if e := db.AutoMigrate(m); e != nil {
				return e
			}
		}
		return nil
	}); e != nil {
		return nil, e
	}
	sql, e := db.DB()
	if e != nil {
		return nil, e
	}
	if e = sql.Ping(); e != nil {
		return nil, e
	}
	return &postgresRepository{db}, nil
}
func (r *postgresRepository) Close() {
	if db, e := r.db.DB(); e == nil {
		if e = db.Close(); e != nil {
			log.Println(e)
		}
	}
}
func (r *postgresRepository) GetCustomerByCustomerID(c context.Context, id string) (*models.Customer, error) {
	var x models.Customer
	e := r.db.WithContext(c).First(&x, "customer_id = ?", id).Error
	return &x, e
}
func (r *postgresRepository) GetCustomerByUserID(c context.Context, id uint64) (*models.Customer, error) {
	var x models.Customer
	e := r.db.WithContext(c).First(&x, "user_id = ?", id).Error
	return &x, e
}
func (r *postgresRepository) SaveCustomer(c context.Context, x *models.Customer) error {
	return r.db.WithContext(c).Create(x).Error
}
func (r *postgresRepository) GetProductByProductID(c context.Context, id string) (*models.Product, error) {
	var x models.Product
	e := r.db.WithContext(c).First(&x, "product_id = ?", id).Error
	return &x, e
}
func (r *postgresRepository) GetProductsByIDs(c context.Context, ids []string) ([]*models.Product, error) {
	var x []*models.Product
	e := r.db.WithContext(c).Find(&x, "product_id IN ?", ids).Error
	return x, e
}
func (r *postgresRepository) SaveProduct(c context.Context, x *models.Product) error {
	return r.db.WithContext(c).Where("product_id = ?", x.ProductID).Assign(x).FirstOrCreate(x).Error
}
func (r *postgresRepository) UpdateProduct(c context.Context, x *models.Product) error {
	return r.db.WithContext(c).Save(x).Error
}
func (r *postgresRepository) DeleteProduct(c context.Context, id string) error {
	return r.db.WithContext(c).Delete(&models.Product{}, "product_id = ?", id).Error
}
func (r *postgresRepository) RegisterTransaction(c context.Context, x *models.Transaction) error {
	return r.db.WithContext(c).Create(x).Error
}
func (r *postgresRepository) UpdateTransaction(c context.Context, x *models.Transaction) error {
	return r.db.WithContext(c).Save(x).Error
}
func (r *postgresRepository) ListTransactions(c context.Context, status string, order, skip, take uint64) ([]*models.Transaction, error) {
	q := r.db.WithContext(c).Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if order > 0 {
		q = q.Where("order_id = ?", order)
	}
	if take > 0 {
		q = q.Offset(int(skip)).Limit(int(take))
	}
	var x []*models.Transaction
	e := q.Find(&x).Error
	return x, e
}
func (r *postgresRepository) GetTransactionByPaymentID(c context.Context, id string) (*models.Transaction, error) {
	var x models.Transaction
	e := r.db.WithContext(c).First(&x, "payment_id = ?", id).Error
	return &x, e
}
func (r *postgresRepository) SaveRefund(c context.Context, x *models.Refund) error {
	return r.db.WithContext(c).Create(x).Error
}
func (r *postgresRepository) GetRefundByKey(c context.Context, k string) (*models.Refund, error) {
	var x models.Refund
	e := r.db.WithContext(c).First(&x, "idempotency_key = ?", k).Error
	return &x, e
}

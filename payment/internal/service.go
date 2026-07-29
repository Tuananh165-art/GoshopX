package internal

import (
	"context"
	"errors"
	"net/http"

	"github.com/dodopayments/dodopayments-go"
	"github.com/rasadov/EcommerceAPI/payment/models"
	"github.com/rasadov/EcommerceAPI/payment/proto/pb"
	"gorm.io/gorm"
)

type Service interface {
	RegisterProduct(ctx context.Context,
		name string, price int64,
		customerId, productId string) error
	UpdateProduct(ctx context.Context, productId string, name string, price int64) error
	DeleteProduct(ctx context.Context, productId string) error

	CreateCustomerPortalSession(ctx context.Context,
		customer *models.Customer) (string, error)
	FindOrCreateCustomer(ctx context.Context,
		userId uint64,
		email, name string) (*models.Customer, error)

	CreateCheckoutSession(ctx context.Context,
		userId uint64,
		customerId string,
		redirect string,
		products []*pb.CartItem, orderId uint64,
	) (checkoutURL string, err error)

	HandlePaymentWebhook(ctx context.Context, w http.ResponseWriter, r *http.Request) (*models.Transaction, error)
}

type paymentService struct {
	client            PaymentClient
	paymentRepository Repository
}

func NewPaymentService(client PaymentClient, paymentRepository Repository) Service {
	return &paymentService{client: client, paymentRepository: paymentRepository}
}

// RegisterProduct - registers product with Dodopayments and returns productId and error.
func (d *paymentService) RegisterProduct(ctx context.Context,
	name string, price int64,
	customerId, productId string) error {

	// We will use USD as currency and Digital Products as tax category for now to keep it simple
	product, err := d.client.CreateProduct(ctx, name, price,
		dodopayments.CurrencyUsd,
		dodopayments.TaxCategoryDigitalProducts,
		customerId, productId)

	if err != nil {
		return err
	}

	return d.paymentRepository.SaveProduct(ctx, &models.Product{
		ProductID:     productId,
		DodoProductID: product.ProductID,
		Price:         product.Price.FixedPrice,
		Currency:      string(product.Price.Currency),
	})
}

func (d *paymentService) UpdateProduct(ctx context.Context,
	productId string,
	name string, price int64) error {
	err := d.client.UpdateProduct(ctx, productId, name, price)
	if err != nil {
		return err
	}

	product, err := d.paymentRepository.GetProductByProductID(ctx, productId)
	if err != nil {
		return err
	}

	if product.Price != price {
		product.Price = price
		err = d.paymentRepository.UpdateProduct(ctx, product)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *paymentService) DeleteProduct(ctx context.Context, productId string) error {
	err := d.client.ArchiveProduct(ctx, productId)
	if err != nil {
		return err
	}

	return d.paymentRepository.DeleteProduct(ctx, productId)
}

// CreateCheckoutSession - returns url to check out page and error.
func (d *paymentService) CreateCheckoutSession(ctx context.Context,
	userId uint64,
	customerId string,
	redirect string,
	products []*pb.CartItem, orderId uint64) (checkoutURL string, err error) {

	productIds := make([]string, len(products))
	productQuantities := make(map[string]uint64, len(products))

	for i, product := range products {
		productIds[i] = product.ProductId
		productQuantities[product.ProductId] = product.Quantity
	}

	modelsProducts, err := d.paymentRepository.GetProductsByIDs(ctx, productIds)
	if err != nil {
		return "", err
	}

	var dodoProducts []dodopayments.CheckoutSessionRequestProductCartParam

	for _, product := range modelsProducts {
		dodoProducts = append(dodoProducts, dodopayments.CheckoutSessionRequestProductCartParam{
			ProductID: dodopayments.F(product.DodoProductID),
			Quantity:  dodopayments.F(int64(productQuantities[product.ProductID])),
		})
	}

	return d.client.CreateCheckoutSession(ctx, userId, customerId, redirect, dodoProducts, orderId)
}

func (d *paymentService) CreateCustomerPortalSession(ctx context.Context, customer *models.Customer) (string, error) {
	customerPortalLink, err := d.client.CreateCustomerSession(ctx, customer.CustomerId)
	if err != nil {
		return "", err
	}
	return customerPortalLink, nil
}

func (d *paymentService) FindOrCreateCustomer(ctx context.Context, userId uint64, email, name string) (*models.Customer, error) {
	existingCustomer, err := d.paymentRepository.GetCustomerByUserID(ctx, userId)

	if err == nil {
		return existingCustomer, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	customer, err := d.client.CreateCustomer(ctx, userId, email, name)

	if err != nil {
		return nil, err
	}

	err = d.paymentRepository.SaveCustomer(ctx, customer)

	return customer, err
}

func (d *paymentService) HandlePaymentWebhook(ctx context.Context, w http.ResponseWriter, r *http.Request) (*models.Transaction, error) {
	updatedTransaction, err := d.client.HandleWebhook(w, r)
	if err != nil {
		return nil, err
	}

	err = d.paymentRepository.RegisterTransaction(ctx, updatedTransaction)
	if err != nil {
		return nil, err
	}

	return updatedTransaction, nil
}

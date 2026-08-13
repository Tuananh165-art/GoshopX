package internal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"github.com/Tuananh165art/GoshopX/payment/vnpay"
	productmodels "github.com/Tuananh165art/GoshopX/product/models"
	"gorm.io/gorm"
)

var ErrUnsupported = errors.New("operation is not supported by VNPAY")

type VNPAYClient interface {
	CreateCheckoutURL(vnpay.CheckoutRequest) (string, error)
	VerifyIPN(url.Values) (*vnpay.Callback, error)
}
type ProductCatalog interface {
	GetProduct(context.Context, string) (*productmodels.Product, error)
}
type Service interface {
	RegisterProduct(context.Context, string, int64, string, string) error
	UpdateProduct(context.Context, string, string, int64) error
	DeleteProduct(context.Context, string) error
	CreateCustomerPortalSession(context.Context, *models.Customer) (string, error)
	FindOrCreateCustomer(context.Context, uint64, string, string) (*models.Customer, error)
	CreateCheckoutSession(context.Context, uint64, string, string, []*pb.CheckoutCartItem, uint64, []string) (string, error)
	HandleVNPAYIPN(context.Context, url.Values) (*models.Transaction, bool, string)
	ListTransactions(context.Context, string, uint64, uint64, uint64) ([]*models.Transaction, error)
	RequestRefund(context.Context, string, string, string) (*models.Refund, error)
	ReconcileTransaction(context.Context, string) (*models.Transaction, error)
	GetProducer() sarama.AsyncProducer
}
type paymentService struct {
	client            VNPAYClient
	paymentRepository Repository
	producer          sarama.AsyncProducer
	catalog           ProductCatalog
}

func NewVNPAYPaymentService(client VNPAYClient, repo Repository, producer sarama.AsyncProducer) Service {
	return NewVNPAYPaymentServiceWithCatalog(client, repo, producer, nil)
}

func NewVNPAYPaymentServiceWithCatalog(client VNPAYClient, repo Repository, producer sarama.AsyncProducer, catalog ProductCatalog) Service {
	return &paymentService{client: client, paymentRepository: repo, producer: producer, catalog: catalog}
}
func (d *paymentService) GetProducer() sarama.AsyncProducer { return d.producer }
func (d *paymentService) RegisterProduct(ctx context.Context, _ string, price int64, _ string, id string) error {
	if price < 0 {
		return errors.New("product price cannot be negative")
	}
	return d.paymentRepository.SaveProduct(ctx, &models.Product{ProductID: id, Price: price, Currency: "VND"})
}
func (d *paymentService) UpdateProduct(ctx context.Context, id, name string, price int64) error {
	_ = name
	p, e := d.paymentRepository.GetProductByProductID(ctx, id)
	if e != nil {
		return e
	}
	p.Price = price
	p.Currency = "VND"
	return d.paymentRepository.UpdateProduct(ctx, p)
}
func (d *paymentService) DeleteProduct(ctx context.Context, id string) error {
	return d.paymentRepository.DeleteProduct(ctx, id)
}
func (d *paymentService) FindOrCreateCustomer(_ context.Context, user uint64, email, name string) (*models.Customer, error) {
	return &models.Customer{UserId: user, BillingEmail: email, BillingName: name}, nil
}
func (d *paymentService) CreateCustomerPortalSession(context.Context, *models.Customer) (string, error) {
	return "", ErrUnsupported
}
func (d *paymentService) RequestRefund(context.Context, string, string, string) (*models.Refund, error) {
	return nil, ErrUnsupported
}
func (d *paymentService) ListTransactions(ctx context.Context, status string, order, skip, take uint64) ([]*models.Transaction, error) {
	return d.paymentRepository.ListTransactions(ctx, status, order, skip, take)
}
func (d *paymentService) ReconcileTransaction(ctx context.Context, id string) (*models.Transaction, error) {
	return d.paymentRepository.GetTransactionByPaymentID(ctx, id)
}
func (d *paymentService) CreateCheckoutSession(ctx context.Context, user uint64, redirect, clientIP string, items []*pb.CheckoutCartItem, order uint64, reservations []string) (string, error) {
	if len(items) == 0 {
		return "", errors.New("checkout requires products")
	}
	ids := make([]string, 0, len(items))
	quantities := map[string]uint64{}
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.ProductId) == "" || item.Quantity == 0 {
			return "", errors.New("invalid checkout product")
		}
		if _, seen := quantities[item.ProductId]; seen {
			return "", errors.New("duplicate checkout product")
		}
		ids = append(ids, item.ProductId)
		quantities[item.ProductId] = item.Quantity
	}
	products, err := d.productsForCheckout(ctx, ids)
	if err != nil {
		return "", err
	}
	if len(products) != len(ids) {
		return "", errors.New("checkout contains unknown products")
	}
	var total int64
	for _, p := range products {
		if p.Currency != "VND" || p.Price <= 0 || quantities[p.ProductID] > uint64(math.MaxInt64/p.Price) {
			return "", errors.New("invalid local product price")
		}
		total += p.Price * int64(quantities[p.ProductID])
		if total < 0 {
			return "", errors.New("checkout total overflow")
		}
	}
	ref, err := newTransactionRef()
	if err != nil {
		return "", err
	}
	tx := &models.Transaction{OrderId: order, UserId: user, PaymentId: ref, TotalPrice: total, Currency: "VND", Status: models.Pending.String(), ReservationIDs: strings.Join(reservations, ",")}
	if err = d.paymentRepository.RegisterTransaction(ctx, tx); err != nil {
		return "", err
	}
	return d.client.CreateCheckoutURL(vnpay.CheckoutRequest{TransactionRef: ref, AmountVND: total, OrderInfo: fmt.Sprintf("Thanh toan don hang %d", order), ReturnURL: redirect, ClientIP: clientIP})
}
func (d *paymentService) productsForCheckout(ctx context.Context, ids []string) ([]*models.Product, error) {
	products, err := d.paymentRepository.GetProductsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*models.Product, len(products))
	for _, product := range products {
		byID[product.ProductID] = product
	}
	for _, id := range ids {
		if d.catalog == nil {
			continue
		}
		catalogProduct, err := d.catalog.GetProduct(ctx, id)
		if err != nil {
			return nil, err
		}
		if catalogProduct == nil || catalogProduct.ID != id {
			continue
		}
		// Product owns the catalog price. It is already denominated in VND, so
		// never apply the DummyJSON USD conversion here. Refreshing the mirror
		// on checkout also replaces old rows created before that conversion was
		// moved into Product.
		price := discountedVNDPrice(catalogProduct.Price, catalogProduct.DiscountPercentage)
		if price <= 0 {
			return nil, errors.New("invalid catalog product price")
		}
		mirror := &models.Product{ProductID: id, Price: price, Currency: "VND"}
		if err := d.paymentRepository.SaveProduct(ctx, mirror); err != nil {
			return nil, err
		}
		byID[id] = mirror
	}
	ordered := make([]*models.Product, 0, len(ids))
	for _, id := range ids {
		if product, ok := byID[id]; ok {
			ordered = append(ordered, product)
		}
	}
	return ordered, nil
}

func roundedVNDPrice(price float64) int64 {
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0
	}
	return int64(math.Round(price))
}

func discountedVNDPrice(price, discountPercentage float64) int64 {
	if math.IsNaN(discountPercentage) || math.IsInf(discountPercentage, 0) || discountPercentage >= 100 {
		return 0
	}
	if discountPercentage > 0 {
		price *= 1 - discountPercentage/100
	}
	return roundedVNDPrice(price)
}

func (d *paymentService) HandleVNPAYIPN(ctx context.Context, values url.Values) (*models.Transaction, bool, string) {
	callback, err := d.client.VerifyIPN(values)
	if err != nil {
		return nil, false, "97"
	}
	tx, err := d.paymentRepository.GetTransactionByPaymentID(ctx, callback.TransactionRef)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, "01"
	}
	if err != nil {
		return nil, false, "99"
	}
	if tx.Status != models.Pending.String() {
		return tx, false, "02"
	}
	if tx.TotalPrice != callback.AmountVND {
		return tx, false, "04"
	}
	status := models.Failed
	if callback.Success {
		status = models.Success
	}
	tx.Status = status.String()
	tx.SettledPrice = callback.AmountVND
	if err := d.paymentRepository.UpdateTransaction(ctx, tx); err != nil {
		return tx, false, "99"
	}
	return tx, true, "00"
}
func newTransactionRef() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}

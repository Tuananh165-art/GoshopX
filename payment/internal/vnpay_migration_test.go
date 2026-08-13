package internal

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"github.com/Tuananh165art/GoshopX/payment/vnpay"
	productmodels "github.com/Tuananh165art/GoshopX/product/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type vnpayMigrationRepository struct {
	products     []*models.Product
	transactions map[string]*models.Transaction
	updates      int
}

func (r *vnpayMigrationRepository) Close() {}
func (r *vnpayMigrationRepository) GetCustomerByCustomerID(context.Context, string) (*models.Customer, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *vnpayMigrationRepository) GetCustomerByUserID(context.Context, uint64) (*models.Customer, error) {
	return nil, gorm.ErrRecordNotFound
}
func (r *vnpayMigrationRepository) SaveCustomer(context.Context, *models.Customer) error { return nil }
func (r *vnpayMigrationRepository) GetProductByProductID(_ context.Context, id string) (*models.Product, error) {
	for _, p := range r.products {
		if p.ProductID == id {
			return p, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (r *vnpayMigrationRepository) GetProductsByIDs(_ context.Context, ids []string) ([]*models.Product, error) {
	var out []*models.Product
	for _, id := range ids {
		p, e := r.GetProductByProductID(context.Background(), id)
		if e == nil {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *vnpayMigrationRepository) SaveProduct(_ context.Context, product *models.Product) error {
	for index, current := range r.products {
		if current.ProductID == product.ProductID {
			r.products[index] = product
			return nil
		}
	}
	r.products = append(r.products, product)
	return nil
}
func (r *vnpayMigrationRepository) UpdateProduct(context.Context, *models.Product) error { return nil }
func (r *vnpayMigrationRepository) DeleteProduct(context.Context, string) error          { return nil }
func (r *vnpayMigrationRepository) RegisterTransaction(_ context.Context, tx *models.Transaction) error {
	if r.transactions == nil {
		r.transactions = map[string]*models.Transaction{}
	}
	if _, ok := r.transactions[tx.PaymentId]; ok {
		return gorm.ErrDuplicatedKey
	}
	r.transactions[tx.PaymentId] = tx
	return nil
}
func (r *vnpayMigrationRepository) UpdateTransaction(_ context.Context, transaction *models.Transaction) error {
	r.updates++
	r.transactions[transaction.PaymentId] = transaction
	return nil
}
func (r *vnpayMigrationRepository) ListTransactions(context.Context, string, uint64, uint64, uint64) ([]*models.Transaction, error) {
	return nil, nil
}
func (r *vnpayMigrationRepository) GetTransactionByPaymentID(_ context.Context, id string) (*models.Transaction, error) {
	tx, ok := r.transactions[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return tx, nil
}
func (r *vnpayMigrationRepository) SaveRefund(context.Context, *models.Refund) error { return nil }
func (r *vnpayMigrationRepository) GetRefundByKey(context.Context, string) (*models.Refund, error) {
	return nil, gorm.ErrRecordNotFound
}

func TestVNPAYCheckoutUsesLocalMirrorAndPersistsPending(t *testing.T) {
	client, err := vnpay.NewClient(vnpay.Config{TmnCode: "TEST", HashSecret: "test-secret", PaymentURL: "https://example.test/pay", Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }})
	require.NoError(t, err)
	repo := &vnpayMigrationRepository{products: []*models.Product{{ProductID: "a", Price: 125000, Currency: "VND"}, {ProductID: "b", Price: 50000, Currency: "VND"}}}
	service := NewVNPAYPaymentService(client, repo, nil)
	checkoutURL, err := service.CreateCheckoutSession(context.Background(), 7, "https://shop.example/return", "203.0.113.10", []*pb.CheckoutCartItem{{ProductId: "a", Quantity: 2}, {ProductId: "b", Quantity: 1}}, 42, []string{"r1"})
	require.NoError(t, err)
	values, err := url.Parse(checkoutURL)
	require.NoError(t, err)
	require.Equal(t, "30000000", values.Query().Get("vnp_Amount"))
	transaction, err := repo.GetTransactionByPaymentID(context.Background(), values.Query().Get("vnp_TxnRef"))
	require.NoError(t, err)
	require.Equal(t, models.Pending.String(), transaction.Status)
	require.Equal(t, int64(300000), transaction.TotalPrice)
}

type catalogStub struct{ product *productmodels.Product }

func (stub catalogStub) GetProduct(context.Context, string) (*productmodels.Product, error) {
	return stub.product, nil
}

func TestVNPAYCheckoutRefreshesStaleMirrorFromDiscountedVNDCatalog(t *testing.T) {
	client, err := vnpay.NewClient(vnpay.Config{TmnCode: "TEST", HashSecret: "test-secret", PaymentURL: "https://example.test/pay"})
	require.NoError(t, err)
	repo := &vnpayMigrationRepository{products: []*models.Product{{ProductID: "1", Price: 874993750, Currency: "VND"}}}
	service := NewVNPAYPaymentServiceWithCatalog(client, repo, nil, catalogStub{product: &productmodels.Product{ID: "1", Price: 34999750, DiscountPercentage: 9.38}})

	checkoutURL, err := service.CreateCheckoutSession(context.Background(), 7, "https://shop.example/return", "203.0.113.10", []*pb.CheckoutCartItem{{ProductId: "1", Quantity: 1}}, 42, nil)
	require.NoError(t, err)
	values, err := url.Parse(checkoutURL)
	require.NoError(t, err)
	require.Equal(t, "3171677300", values.Query().Get("vnp_Amount"))
	require.Len(t, repo.products, 1)
	require.Equal(t, int64(31716773), repo.products[0].Price)
	transaction, err := repo.GetTransactionByPaymentID(context.Background(), values.Query().Get("vnp_TxnRef"))
	require.NoError(t, err)
	require.Equal(t, int64(31716773), transaction.TotalPrice)
}

func TestVNPAYProviderOnlyOperationsAreUnsupported(t *testing.T) {
	client, err := vnpay.NewClient(vnpay.Config{TmnCode: "TEST", HashSecret: "test-secret", PaymentURL: "https://example.test/pay"})
	require.NoError(t, err)
	service := NewVNPAYPaymentService(client, &vnpayMigrationRepository{}, nil)
	_, err = service.CreateCustomerPortalSession(context.Background(), &models.Customer{})
	require.ErrorContains(t, err, "not supported")
	_, err = service.RequestRefund(context.Background(), "payment", "reason", "key")
	require.ErrorContains(t, err, "not supported")
}

type ipnStub struct{ callback *vnpay.Callback }

func (s ipnStub) CreateCheckoutURL(vnpay.CheckoutRequest) (string, error) { return "", nil }
func (s ipnStub) VerifyIPN(url.Values) (*vnpay.Callback, error)           { return s.callback, nil }

func TestVNPAYIPNTransitionsPendingOnlyOnce(t *testing.T) {
	repo := &vnpayMigrationRepository{transactions: map[string]*models.Transaction{"ref": {PaymentId: "ref", TotalPrice: 125000, Status: models.Pending.String()}}}
	service := NewVNPAYPaymentService(ipnStub{callback: &vnpay.Callback{TransactionRef: "ref", AmountVND: 125000, Success: true}}, repo, nil)
	transaction, first, code := service.HandleVNPAYIPN(context.Background(), url.Values{})
	require.Equal(t, "00", code)
	require.True(t, first)
	require.Equal(t, models.Success.String(), transaction.Status)
	_, first, code = service.HandleVNPAYIPN(context.Background(), url.Values{})
	require.Equal(t, "02", code)
	require.False(t, first)
	require.Equal(t, 1, repo.updates)
}

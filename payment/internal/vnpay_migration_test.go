package internal

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	ordermodels "github.com/Tuananh165art/GoshopX/order/models"
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
	registerErr  error
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
	if r.registerErr != nil {
		return r.registerErr
	}
	if r.transactions == nil {
		r.transactions = map[string]*models.Transaction{}
	}
	if _, ok := r.transactions[tx.PaymentId]; ok {
		return gorm.ErrDuplicatedKey
	}
	tx.ID = uint(len(r.transactions) + 1)
	r.transactions[tx.PaymentId] = tx
	return nil
}
func (r *vnpayMigrationRepository) GetPendingTransactionByCheckoutKey(_ context.Context, key string) (*models.Transaction, error) {
	for _, tx := range r.transactions {
		if tx.CheckoutKey != nil && *tx.CheckoutKey == key && tx.Status == models.Pending.String() {
			return tx, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
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

type orderReaderStub struct{ order *ordermodels.Order }

func (stub orderReaderStub) GetOrder(context.Context, uint64) (*ordermodels.Order, error) {
	return stub.order, nil
}

func TestVNPAYCheckoutConsumesOrderOwnedDiscountedTotal(t *testing.T) {
	client, err := vnpay.NewClient(vnpay.Config{TmnCode: "TEST", HashSecret: "test-secret", PaymentURL: "https://example.test/pay"})
	require.NoError(t, err)
	repo := &vnpayMigrationRepository{products: []*models.Product{{ProductID: "p1", Price: 2_000_000, Currency: "VND"}}}
	order := &ordermodels.Order{ID: 42, AccountID: 7, Status: "pending", SubtotalVND: 2_000_000, DiscountVND: 100_000, TotalPriceVND: 1_900_000, Products: []*ordermodels.OrderedProduct{{ID: "p1", Quantity: 1}}}
	service := NewVNPAYPaymentServiceWithCommerce(client, repo, nil, nil, orderReaderStub{order: order})
	checkoutURL, err := service.CreateCheckoutSession(context.Background(), 7, "https://shop.example/return", "203.0.113.10", []*pb.CheckoutCartItem{{ProductId: "p1", Quantity: 1}}, 42, nil)
	require.NoError(t, err)
	values, err := url.Parse(checkoutURL)
	require.NoError(t, err)
	require.Equal(t, "190000000", values.Query().Get("vnp_Amount"))
	tx, err := repo.GetTransactionByPaymentID(context.Background(), values.Query().Get("vnp_TxnRef"))
	require.NoError(t, err)
	require.Equal(t, int64(1_900_000), tx.TotalPrice)
}

type checkoutFailureClient struct {
	calls []vnpay.CheckoutRequest
	fail  bool
}

func (c *checkoutFailureClient) CreateCheckoutURL(request vnpay.CheckoutRequest) (string, error) {
	c.calls = append(c.calls, request)
	if c.fail {
		c.fail = false
		return "", errors.New("temporary provider failure")
	}
	return "https://example.test/pay?vnp_TxnRef=" + request.TransactionRef, nil
}
func (*checkoutFailureClient) VerifyIPN(url.Values) (*vnpay.Callback, error) {
	return nil, errors.New("unused")
}

func TestVNPAYCheckoutRetryReusesPendingOrderTransaction(t *testing.T) {
	repo := &vnpayMigrationRepository{products: []*models.Product{{ProductID: "p1", Price: 100_000, Currency: "VND"}}}
	client := &checkoutFailureClient{fail: true}
	service := NewVNPAYPaymentService(client, repo, nil)
	items := []*pb.CheckoutCartItem{{ProductId: "p1", Quantity: 1}}
	_, err := service.CreateCheckoutSession(context.Background(), 7, "https://shop.example/return", "203.0.113.10", items, 42, nil)
	require.ErrorContains(t, err, "temporary provider failure")
	url, err := service.CreateCheckoutSession(context.Background(), 7, "https://shop.example/return", "203.0.113.10", items, 42, nil)
	require.NoError(t, err)
	require.Contains(t, url, client.calls[0].TransactionRef)
	require.Equal(t, client.calls[0].TransactionRef, client.calls[1].TransactionRef)
	require.Len(t, repo.transactions, 1)
}

func TestVNPAYCheckoutReturnsTransactionPersistenceErrorWithoutWinner(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &vnpayMigrationRepository{products: []*models.Product{{ProductID: "p1", Price: 100_000, Currency: "VND"}}, registerErr: wantErr}
	service := NewVNPAYPaymentService(&checkoutFailureClient{}, repo, nil)
	_, err := service.CreateCheckoutSession(context.Background(), 7, "https://shop.example/return", "203.0.113.10", []*pb.CheckoutCartItem{{ProductId: "p1", Quantity: 1}}, 42, nil)
	require.ErrorIs(t, err, wantErr)
	require.Empty(t, repo.transactions)
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

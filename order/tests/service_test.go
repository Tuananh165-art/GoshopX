package tests

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/order/internal"
	"github.com/Tuananh165art/GoshopX/order/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) PutOrder(ctx context.Context, order *models.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockRepository) PutOrderWithCoupon(ctx context.Context, order *models.Order, couponCode string) error {
	args := m.Called(ctx, order, couponCode)
	return args.Error(0)
}

func (m *MockRepository) CreateCheckoutQuote(ctx context.Context, quote *models.CheckoutQuoteRecord) error {
	args := m.Called(ctx, quote)
	return args.Error(0)
}

func (m *MockRepository) PutOrderWithQuote(ctx context.Context, order *models.Order, quoteID, itemsHash string, cartVersion uint64) (bool, error) {
	args := m.Called(ctx, order, quoteID, itemsHash, cartVersion)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepository) PreviewCoupon(ctx context.Context, accountID uint64, subtotalVND int64, couponCode string) (int64, error) {
	args := m.Called(ctx, accountID, subtotalVND, couponCode)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) GetOrdersForAccount(ctx context.Context, accountId uint64) ([]*models.Order, error) {
	args := m.Called(ctx, accountId)
	return args.Get(0).([]*models.Order), args.Error(1)
}

func (m *MockRepository) UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, status string) error {
	args := m.Called(ctx, orderId, status)
	return args.Error(0)
}

func (m *MockRepository) ListOrders(ctx context.Context, status, paymentStatus string, accountID, skip, take uint64) ([]*models.Order, error) {
	args := m.Called(ctx, status, paymentStatus, accountID, skip, take)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Order), args.Error(1)
}

func (m *MockRepository) GetOrderByID(ctx context.Context, orderID uint64) (*models.Order, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockRepository) CancelOrder(ctx context.Context, orderID uint64, reason string) (*models.Order, error) {
	args := m.Called(ctx, orderID, reason)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *MockRepository) Close() {}

type stubAsyncProducer struct {
	input chan *sarama.ProducerMessage
}

func newStubAsyncProducer() *stubAsyncProducer {
	p := &stubAsyncProducer{input: make(chan *sarama.ProducerMessage, 100)}
	go func() {
		for range p.input {
		}
	}()
	return p
}

func (p *stubAsyncProducer) AsyncClose() {}

func (p *stubAsyncProducer) Close() error { return nil }

func (p *stubAsyncProducer) Input() chan<- *sarama.ProducerMessage { return p.input }

func (p *stubAsyncProducer) Successes() <-chan *sarama.ProducerMessage { return nil }

func (p *stubAsyncProducer) Errors() <-chan *sarama.ProducerError { return nil }

func (p *stubAsyncProducer) IsTransactional() bool { return false }

func (p *stubAsyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }

func (p *stubAsyncProducer) BeginTxn() error { return nil }

func (p *stubAsyncProducer) CommitTxn() error { return nil }

func (p *stubAsyncProducer) AbortTxn() error { return nil }

func (p *stubAsyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}

func (p *stubAsyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}

func TestOrderService_PostOrder(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockRepository)
	producer := newStubAsyncProducer()
	service := internal.NewOrderService(mockRepo, producer)

	t.Run("Successful order creation", func(t *testing.T) {
		accountID := uint64(1)
		totalPrice := 99.99
		products := []*models.OrderedProduct{
			{ID: "product-1", Quantity: 1},
			{ID: "product-2", Quantity: 2},
		}

		mockRepo.On("PutOrder", ctx, mock.AnythingOfType("*models.Order")).Return(nil).Once()

		result, err := service.PostOrder(ctx, accountID, totalPrice, products, false)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, accountID, result.AccountID)
		assert.Equal(t, totalPrice, result.TotalPrice)
		assert.Equal(t, products, result.Products)
		mockRepo.AssertExpectations(t)

		time.Sleep(10 * time.Millisecond)
	})

	t.Run("Repository error", func(t *testing.T) {
		accountID := uint64(2)
		totalPrice := 50.00
		products := []*models.OrderedProduct{
			{ID: "product-3", Quantity: 1},
		}

		mockRepo.On("PutOrder", ctx, mock.AnythingOfType("*models.Order")).Return(errors.New("database error")).Once()

		result, err := service.PostOrder(ctx, accountID, totalPrice, products, false)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "database error", err.Error())
		mockRepo.AssertExpectations(t)
	})
}

func TestOrderService_PreviewCheckoutReturnsExpiringIntegerVNDQuote(t *testing.T) {
	repository := new(MockRepository)
	repository.On("PreviewCoupon", mock.Anything, uint64(42), int64(2_000_000), "LAB10").Return(int64(100_000), nil).Once()
	repository.On("CreateCheckoutQuote", mock.Anything, mock.MatchedBy(func(quote *models.CheckoutQuoteRecord) bool {
		return quote.AccountID == 42 && quote.ItemsHash == models.CheckoutItemsHash([]*models.OrderedProduct{{ID: "p1", Quantity: 1}})
	})).Return(nil).Once()
	service := internal.NewOrderService(repository, nil)
	before := time.Now().UTC()
	quote, err := service.PreviewCheckout(context.Background(), 42, 2_000_000, []*models.OrderedProduct{{ID: "p1", Quantity: 1}}, " lab10 ", 17)
	assert.NoError(t, err)
	assert.Equal(t, int64(2_000_000), quote.SubtotalVND)
	assert.Equal(t, int64(100_000), quote.DiscountVND)
	assert.Equal(t, int64(1_900_000), quote.TotalPriceVND)
	assert.Equal(t, "LAB10", quote.CouponCode)
	assert.Len(t, quote.ID, 48)
	assert.Equal(t, uint64(17), quote.CartVersion)
	assert.WithinDuration(t, before.Add(10*time.Minute), quote.ExpiresAt, time.Second)
	repository.AssertExpectations(t)
}

func TestOrderServiceRejectsNonFiniteOrNegativeVNDTotal(t *testing.T) {
	service := internal.NewOrderService(new(MockRepository), nil)
	for _, total := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), float64(math.MaxInt64)} {
		if _, err := service.PostOrder(context.Background(), 1, total, nil, false); !errors.Is(err, internal.ErrInvalidOrderTotal) {
			t.Fatalf("PostOrder(%v) error = %v, want ErrInvalidOrderTotal", total, err)
		}
	}
}

func TestOrderService_GetOrdersForAccount(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockRepository)
	producer := newStubAsyncProducer()
	service := internal.NewOrderService(mockRepo, producer)

	t.Run("Successful get orders", func(t *testing.T) {
		accountID := uint64(1)
		orders := []*models.Order{
			{ID: 1, AccountID: accountID, TotalPrice: 99.99},
			{ID: 2, AccountID: accountID, TotalPrice: 49.50},
		}

		mockRepo.On("GetOrdersForAccount", ctx, accountID).Return(orders, nil).Once()

		result, err := service.GetOrdersForAccount(ctx, accountID)

		assert.NoError(t, err)
		assert.Equal(t, orders, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Repository error", func(t *testing.T) {
		accountID := uint64(1)

		mockRepo.On("GetOrdersForAccount", ctx, accountID).Return([]*models.Order(nil), errors.New("not found")).Once()

		result, err := service.GetOrdersForAccount(ctx, accountID)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestOrderService_UpdateOrderPaymentStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockRepository)
	producer := newStubAsyncProducer()
	service := internal.NewOrderService(mockRepo, producer)

	t.Run("Successful payment status update", func(t *testing.T) {
		orderID := uint64(1)
		status := "paid"

		mockRepo.On("UpdateOrderPaymentStatus", ctx, orderID, status).Return(nil).Once()
		mockRepo.On("GetOrderByID", mock.Anything, orderID).Return(&models.Order{ID: uint(orderID), AccountID: 7}, nil).Maybe()

		err := service.UpdateOrderPaymentStatus(ctx, orderID, status)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Repository error", func(t *testing.T) {
		orderID := uint64(1)
		status := "failed"

		mockRepo.On("UpdateOrderPaymentStatus", ctx, orderID, status).Return(errors.New("update failed")).Once()

		err := service.UpdateOrderPaymentStatus(ctx, orderID, status)

		assert.Error(t, err)
		assert.Equal(t, "update failed", err.Error())
		mockRepo.AssertExpectations(t)
	})
}

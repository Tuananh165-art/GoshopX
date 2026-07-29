package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/cart/internal"
	"github.com/Tuananh165art/GoshopX/cart/models"
	inventorymodels "github.com/Tuananh165art/GoshopX/inventory/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetCart(ctx context.Context, accountID uint64) (*models.Cart, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Cart), args.Error(1)
}

func (m *MockRepository) SaveCart(ctx context.Context, cart *models.Cart, ttl time.Duration) error {
	args := m.Called(ctx, cart, ttl)
	return args.Error(0)
}

func (m *MockRepository) DeleteCart(ctx context.Context, accountID uint64) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

type MockInventoryClient struct {
	mock.Mock
}

func (m *MockInventoryClient) ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, ttl time.Duration, source string) (*inventorymodels.StockReservation, *inventorymodels.Availability, error) {
	args := m.Called(ctx, reservationID, accountID, productID, quantity, ttl, source)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*inventorymodels.StockReservation), args.Get(1).(*inventorymodels.Availability), args.Error(2)
}

func (m *MockInventoryClient) ReleaseReservation(ctx context.Context, reservationID string) (*inventorymodels.StockReservation, *inventorymodels.Availability, error) {
	args := m.Called(ctx, reservationID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*inventorymodels.StockReservation), args.Get(1).(*inventorymodels.Availability), args.Error(2)
}

type stubAsyncProducer struct {
	input chan *sarama.ProducerMessage
}

func newStubAsyncProducer() *stubAsyncProducer {
	p := &stubAsyncProducer{input: make(chan *sarama.ProducerMessage, 32)}
	go func() {
		for range p.input {
		}
	}()
	return p
}

func (p *stubAsyncProducer) AsyncClose()                               {}
func (p *stubAsyncProducer) Close() error                              { return nil }
func (p *stubAsyncProducer) Input() chan<- *sarama.ProducerMessage     { return p.input }
func (p *stubAsyncProducer) Successes() <-chan *sarama.ProducerMessage { return nil }
func (p *stubAsyncProducer) Errors() <-chan *sarama.ProducerError      { return nil }
func (p *stubAsyncProducer) IsTransactional() bool                     { return false }
func (p *stubAsyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag   { return 0 }
func (p *stubAsyncProducer) BeginTxn() error                           { return nil }
func (p *stubAsyncProducer) CommitTxn() error                          { return nil }
func (p *stubAsyncProducer) AbortTxn() error                           { return nil }
func (p *stubAsyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}
func (p *stubAsyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}

func TestCartService_UpsertCartItem(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())

	repo.On("GetCart", ctx, uint64(1)).Return((*models.Cart)(nil), internal.ErrCartNotFound).Once()
	inv.On("ReserveStock", ctx, "", uint64(1), "product-1", int32(2), 15*time.Minute, "cart").Return(
		&inventorymodels.StockReservation{ReservationID: "r1", ProductID: "product-1", Quantity: 2, ExpiresAt: time.Now().UTC().Add(15 * time.Minute)},
		&inventorymodels.Availability{ProductID: "product-1", AvailableQuantity: 8},
		nil,
	).Once()
	repo.On("SaveCart", ctx, mock.AnythingOfType("*models.Cart"), 15*time.Minute).Return(nil).Once()

	cart, err := service.UpsertCartItem(ctx, 1, "product-1", 2)

	assert.NoError(t, err)
	assert.Len(t, cart.Items, 1)
	assert.Equal(t, "r1", cart.Items[0].ReservationID)
}

func TestCartService_RemoveCartItem(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())

	existing := &models.Cart{
		AccountID: 1,
		Items: []models.CartItem{{
			ProductID:     "product-1",
			Quantity:      1,
			ReservationID: "r1",
			ReservedUntil: time.Now().UTC().Add(15 * time.Minute),
		}},
	}
	repo.On("GetCart", ctx, uint64(1)).Return(existing, nil).Once()
	inv.On("ReleaseReservation", ctx, "r1").Return(
		&inventorymodels.StockReservation{ReservationID: "r1"},
		&inventorymodels.Availability{},
		nil,
	).Once()
	repo.On("DeleteCart", ctx, uint64(1)).Return(nil).Once()

	cart, err := service.RemoveCartItem(ctx, 1, "product-1")

	assert.NoError(t, err)
	assert.Len(t, cart.Items, 0)
}

func TestCartService_ClearCart(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())

	existing := &models.Cart{
		AccountID: 1,
		Items: []models.CartItem{{
			ProductID:     "product-1",
			Quantity:      1,
			ReservationID: "r1",
			ReservedUntil: time.Now().UTC().Add(15 * time.Minute),
		}},
	}
	repo.On("GetCart", ctx, uint64(1)).Return(existing, nil).Once()
	inv.On("ReleaseReservation", ctx, "r1").Return(
		&inventorymodels.StockReservation{ReservationID: "r1"},
		&inventorymodels.Availability{},
		nil,
	).Once()
	repo.On("DeleteCart", ctx, uint64(1)).Return(nil).Once()

	err := service.ClearCart(ctx, 1)

	assert.NoError(t, err)
}

func TestCartService_PrepareCheckoutFiltersExpiredReservations(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())

	existing := &models.Cart{
		AccountID: 1,
		Items: []models.CartItem{
			{ProductID: "old", Quantity: 1, ReservationID: "r-old", ReservedUntil: time.Now().UTC().Add(-time.Minute)},
			{ProductID: "new", Quantity: 2, ReservationID: "r-new", ReservedUntil: time.Now().UTC().Add(time.Minute)},
		},
	}
	repo.On("GetCart", ctx, uint64(1)).Return(existing, nil).Once()

	cart, err := service.PrepareCheckout(ctx, 1)

	assert.NoError(t, err)
	assert.Len(t, cart.Items, 1)
	assert.Equal(t, "new", cart.Items[0].ProductID)
}

func TestCartService_RejectsInvalidQuantity(t *testing.T) {
	service := internal.NewCartService(new(MockRepository), new(MockInventoryClient), newStubAsyncProducer())

	cart, err := service.UpsertCartItem(context.Background(), 1, "product-1", 0)

	assert.ErrorIs(t, err, internal.ErrInvalidCartQuantity)
	assert.Nil(t, cart)
}

func TestCartService_PropagatesInventoryError(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())

	repo.On("GetCart", ctx, uint64(1)).Return((*models.Cart)(nil), internal.ErrCartNotFound).Once()
	inv.On("ReserveStock", ctx, "", uint64(1), "product-1", int32(2), 15*time.Minute, "cart").Return(
		(*inventorymodels.StockReservation)(nil),
		(*inventorymodels.Availability)(nil),
		errors.New("insufficient stock"),
	).Once()

	cart, err := service.UpsertCartItem(ctx, 1, "product-1", 2)

	assert.EqualError(t, err, "insufficient stock")
	assert.Nil(t, cart)
}


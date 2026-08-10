package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/inventory/internal"
	"github.com/Tuananh165art/GoshopX/inventory/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Close() {}

func (m *MockRepository) UpsertStock(ctx context.Context, productID string, quantity, reorderLevel int32) (*models.Stock, error) {
	args := m.Called(ctx, productID, quantity, reorderLevel)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Stock), args.Error(1)
}

func (m *MockRepository) GetAvailability(ctx context.Context, productID string) (*models.Availability, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Availability), args.Error(1)
}

func (m *MockRepository) ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, expiresAt time.Time, source string) (*models.StockReservation, *models.Availability, error) {
	args := m.Called(ctx, reservationID, accountID, productID, quantity, mock.AnythingOfType("time.Time"), source)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*models.StockReservation), args.Get(1).(*models.Availability), args.Error(2)
}

func (m *MockRepository) ReleaseReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	args := m.Called(ctx, reservationID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*models.StockReservation), args.Get(1).(*models.Availability), args.Error(2)
}

func (m *MockRepository) CommitReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	args := m.Called(ctx, reservationID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*models.StockReservation), args.Get(1).(*models.Availability), args.Error(2)
}

func (m *MockRepository) ListLowStock(ctx context.Context, limit uint64) ([]*models.Stock, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*models.Stock), args.Error(1)
}

func (m *MockRepository) CleanupExpiredReservations(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) AdjustStock(ctx context.Context, productID string, delta, reorderLevel int32) (*models.Stock, error) {
	args := m.Called(ctx, productID, delta, reorderLevel)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Stock), args.Error(1)
}
func (m *MockRepository) ListReservations(ctx context.Context, status string, skip, take uint64) ([]*models.StockReservation, error) {
	args := m.Called(ctx, status, skip, take)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.StockReservation), args.Error(1)
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

func TestInventoryService_UpsertStock(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	service := internal.NewInventoryService(repo, newStubAsyncProducer())

	stock := &models.Stock{ProductID: "product-1", Quantity: 10, ReorderLevel: 2}
	availability := &models.Availability{ProductID: "product-1", TotalQuantity: 10, ReservedQuantity: 0, AvailableQuantity: 10}

	repo.On("UpsertStock", ctx, "product-1", int32(10), int32(2)).Return(stock, nil).Once()
	repo.On("GetAvailability", ctx, "product-1").Return(availability, nil).Once()

	gotStock, gotAvailability, err := service.UpsertStock(ctx, "product-1", 10, 2)

	assert.NoError(t, err)
	assert.Equal(t, stock, gotStock)
	assert.Equal(t, availability, gotAvailability)
	repo.AssertExpectations(t)
}

func TestInventoryService_ReserveStock(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	service := internal.NewInventoryService(repo, newStubAsyncProducer())

	reservation := &models.StockReservation{ReservationID: "r1", ProductID: "product-1", Quantity: 2, Status: models.ReservationActive}
	availability := &models.Availability{ProductID: "product-1", TotalQuantity: 10, ReservedQuantity: 2, AvailableQuantity: 8}

	repo.On("ReserveStock", ctx, mock.Anything, uint64(1), "product-1", int32(2), mock.Anything, "cart").Return(reservation, availability, nil).Once()

	gotReservation, gotAvailability, err := service.ReserveStock(ctx, "", 1, "product-1", 2, 15*time.Minute, "cart")

	assert.NoError(t, err)
	assert.NotEmpty(t, gotReservation.ReservationID)
	assert.Equal(t, availability, gotAvailability)
	repo.AssertExpectations(t)
}

func TestInventoryService_ReserveStockRejectsInvalidQuantity(t *testing.T) {
	service := internal.NewInventoryService(new(MockRepository), newStubAsyncProducer())

	reservation, availability, err := service.ReserveStock(context.Background(), "", 1, "product-1", 0, time.Minute, "cart")

	assert.ErrorIs(t, err, internal.ErrInvalidQuantity)
	assert.Nil(t, reservation)
	assert.Nil(t, availability)
}

func TestInventoryService_CommitReservation(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	service := internal.NewInventoryService(repo, newStubAsyncProducer())

	reservation := &models.StockReservation{ReservationID: "r1", ProductID: "product-1", Quantity: 2, Status: models.ReservationCommitted}
	availability := &models.Availability{ProductID: "product-1", TotalQuantity: 8, ReservedQuantity: 0, AvailableQuantity: 8}
	repo.On("CommitReservation", ctx, "r1").Return(reservation, availability, nil).Once()

	gotReservation, gotAvailability, err := service.CommitReservation(ctx, "r1")

	assert.NoError(t, err)
	assert.Equal(t, reservation, gotReservation)
	assert.Equal(t, availability, gotAvailability)
	repo.AssertExpectations(t)
}

func TestInventoryService_ReleaseReservation(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	service := internal.NewInventoryService(repo, newStubAsyncProducer())

	reservation := &models.StockReservation{ReservationID: "r1", ProductID: "product-1", Quantity: 2, Status: models.ReservationReleased}
	availability := &models.Availability{ProductID: "product-1", TotalQuantity: 10, ReservedQuantity: 0, AvailableQuantity: 10}
	repo.On("ReleaseReservation", ctx, "r1").Return(reservation, availability, nil).Once()

	gotReservation, gotAvailability, err := service.ReleaseReservation(ctx, "r1")

	assert.NoError(t, err)
	assert.Equal(t, reservation, gotReservation)
	assert.Equal(t, availability, gotAvailability)
	repo.AssertExpectations(t)
}

func TestInventoryService_UpsertStockPropagatesRepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	service := internal.NewInventoryService(repo, newStubAsyncProducer())

	repo.On("UpsertStock", ctx, "product-1", int32(10), int32(2)).Return((*models.Stock)(nil), errors.New("db error")).Once()

	stock, availability, err := service.UpsertStock(ctx, "product-1", 10, 2)

	assert.EqualError(t, err, "db error")
	assert.Nil(t, stock)
	assert.Nil(t, availability)
	repo.AssertExpectations(t)
}

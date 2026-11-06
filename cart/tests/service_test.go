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
	cartLockGranted *bool
	cartLockErr     error
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

func (m *MockRepository) AcquireCartLock(context.Context, uint64, string, time.Duration) (bool, error) {
	if m.cartLockGranted != nil {
		return *m.cartLockGranted, m.cartLockErr
	}
	return true, m.cartLockErr
}

func (m *MockRepository) ReleaseCartLock(context.Context, uint64, string) error { return nil }

func (m *MockRepository) GetCartProposal(ctx context.Context, accountID uint64, key string) (*models.CartProposal, error) {
	args := m.Called(ctx, accountID, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CartProposal), args.Error(1)
}

func (m *MockRepository) GetCartProposalByID(ctx context.Context, accountID uint64, id string) (*models.CartProposal, error) {
	args := m.Called(ctx, accountID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CartProposal), args.Error(1)
}

func (m *MockRepository) SaveCartProposal(ctx context.Context, proposal *models.CartProposal, ttl time.Duration) error {
	return m.Called(ctx, proposal, ttl).Error(0)
}

func (m *MockRepository) SaveCartAndProposal(ctx context.Context, cart *models.Cart, proposal *models.CartProposal, ttl time.Duration) error {
	return m.Called(ctx, cart, proposal, ttl).Error(0)
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
	assert.Equal(t, uint64(1), cart.Version)
}

func TestCartService_CreateAndConfirmCartProposalIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())

	proposal := &models.CartProposal{IdempotencyKey: "request-1", AccountID: 7, BaseVersion: 3, Items: []models.ProposalItem{{ProductID: "p1", Quantity: 2}}, ExpiresAt: time.Now().UTC().Add(time.Minute)}
	repo.On("GetCartProposal", mock.Anything, uint64(7), "request-1").Return((*models.CartProposal)(nil), internal.ErrCartNotFound).Once()
	repo.On("GetCart", mock.Anything, uint64(7)).Return(&models.Cart{AccountID: 7, Version: 3, Items: []models.CartItem{}}, nil).Once()
	repo.On("SaveCartProposal", mock.Anything, mock.MatchedBy(func(got *models.CartProposal) bool {
		return got.IdempotencyKey == proposal.IdempotencyKey && got.BaseVersion == 3
	}), mock.AnythingOfType("time.Duration")).Return(nil).Once()
	created, err := service.CreateCartProposal(ctx, 7, 3, "request-1", proposal.Items)
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), created.BaseVersion)
	proposal.ID = created.ID

	repo.On("GetCartProposalByID", mock.Anything, uint64(7), proposal.ID).Return(proposal, nil).Twice()
	repo.On("GetCart", mock.Anything, uint64(7)).Return(&models.Cart{AccountID: 7, Version: 3, Items: []models.CartItem{}}, nil).Once()
	inv.On("ReserveStock", mock.Anything, mock.AnythingOfType("string"), uint64(7), "p1", int32(2), mock.AnythingOfType("time.Duration"), "cart").Return(
		&inventorymodels.StockReservation{ReservationID: "reservation-1", ProductID: "p1", Quantity: 2, ExpiresAt: time.Now().UTC().Add(15 * time.Minute)},
		&inventorymodels.Availability{ProductID: "p1", AvailableQuantity: 8}, nil,
	).Once()
	repo.On("SaveCartAndProposal", mock.Anything, mock.MatchedBy(func(cart *models.Cart) bool { return cart.Version == 4 && len(cart.Items) == 1 }), mock.MatchedBy(func(got *models.CartProposal) bool { return got.Result != nil && got.Result.Version == 4 }), mock.AnythingOfType("time.Duration")).Return(nil).Once()
	first, err := service.ConfirmCartProposal(ctx, 7, proposal.ID)
	assert.NoError(t, err)
	assert.Equal(t, uint64(4), first.Version)
	second, err := service.ConfirmCartProposal(ctx, 7, proposal.ID)
	assert.NoError(t, err)
	assert.Equal(t, first.Version, second.Version)
	assert.Len(t, second.Items, 1)
	repo.AssertExpectations(t)
	inv.AssertExpectations(t)
}

func TestCartService_ProposalMergesCartAndRestoresReservationOnPartialFailure(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())
	proposal := &models.CartProposal{ID: "proposal-merge", IdempotencyKey: "key-merge", AccountID: 8, BaseVersion: 4, Items: []models.ProposalItem{{ProductID: "p1", Quantity: 2}, {ProductID: "p2", Quantity: 1}}, ExpiresAt: time.Now().UTC().Add(time.Minute)}
	repo.On("GetCartProposalByID", mock.Anything, uint64(8), proposal.ID).Return(proposal, nil).Once()
	repo.On("GetCart", mock.Anything, uint64(8)).Return(&models.Cart{AccountID: 8, Version: 4, Items: []models.CartItem{{ProductID: "p1", Quantity: 1, ReservationID: "old-r1", ReservedUntil: time.Now().UTC().Add(time.Minute)}}}, nil).Once()
	inv.On("ReserveStock", mock.Anything, "old-r1", uint64(8), "p1", int32(3), mock.AnythingOfType("time.Duration"), "cart").Return(
		&inventorymodels.StockReservation{ReservationID: "old-r1", ProductID: "p1", Quantity: 3, ExpiresAt: time.Now().UTC().Add(time.Minute)}, &inventorymodels.Availability{}, nil).Once()
	inv.On("ReserveStock", mock.Anything, mock.AnythingOfType("string"), uint64(8), "p2", int32(1), mock.AnythingOfType("time.Duration"), "cart").Return(nil, nil, errors.New("out of stock")).Once()
	inv.On("ReserveStock", mock.Anything, "old-r1", uint64(8), "p1", int32(1), mock.AnythingOfType("time.Duration"), "cart").Return(
		&inventorymodels.StockReservation{ReservationID: "old-r1", ProductID: "p1", Quantity: 1}, &inventorymodels.Availability{}, nil).Once()

	cart, err := service.ConfirmCartProposal(ctx, 8, proposal.ID)
	assert.Nil(t, cart)
	assert.EqualError(t, err, "out of stock")
	repo.AssertNotCalled(t, "SaveCartAndProposal", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	inv.AssertExpectations(t)
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
	repo.On("SaveCart", ctx, mock.MatchedBy(func(cart *models.Cart) bool {
		return len(cart.Items) == 0 && cart.Version == 1
	}), 15*time.Minute).Return(nil).Once()

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
	repo.On("SaveCart", ctx, mock.MatchedBy(func(cart *models.Cart) bool {
		return len(cart.Items) == 0 && cart.Version == 1
	}), 15*time.Minute).Return(nil).Once()

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

func TestCartService_PrepareCheckoutChecksCartVersion(t *testing.T) {
	ctx := context.Background()

	t.Run("matching version", func(t *testing.T) {
		repo := new(MockRepository)
		service := internal.NewCartService(repo, new(MockInventoryClient), newStubAsyncProducer())
		existing := &models.Cart{AccountID: 1, Version: 7, Items: []models.CartItem{{ProductID: "p1", Quantity: 1}}}
		repo.On("GetCart", ctx, uint64(1)).Return(existing, nil).Once()

		got, err := service.PrepareCheckoutAtVersion(ctx, 1, 7)

		assert.NoError(t, err)
		assert.Equal(t, uint64(7), got.Version)
		repo.AssertExpectations(t)
	})

	t.Run("stale version", func(t *testing.T) {
		repo := new(MockRepository)
		service := internal.NewCartService(repo, new(MockInventoryClient), newStubAsyncProducer())
		existing := &models.Cart{AccountID: 1, Version: 8, Items: []models.CartItem{{ProductID: "p1", Quantity: 1}}}
		repo.On("GetCart", ctx, uint64(1)).Return(existing, nil).Once()

		got, err := service.PrepareCheckoutAtVersion(ctx, 1, 7)

		assert.ErrorIs(t, err, internal.ErrCartVersionConflict)
		assert.Nil(t, got)
		repo.AssertExpectations(t)
	})
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

func TestCartService_AddCartItemAccumulatesExistingQuantity(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())
	existing := &models.Cart{AccountID: 1, Items: []models.CartItem{{ProductID: "product-1", Quantity: 2, ReservationID: "r1", ReservedUntil: time.Now().UTC().Add(15 * time.Minute)}}}
	repo.On("GetCart", ctx, uint64(1)).Return(existing, nil).Once()
	inv.On("ReserveStock", ctx, "r1", uint64(1), "product-1", int32(3), 15*time.Minute, "cart").Return(
		&inventorymodels.StockReservation{ReservationID: "r1", ProductID: "product-1", Quantity: 3, ExpiresAt: time.Now().UTC().Add(15 * time.Minute)},
		&inventorymodels.Availability{ProductID: "product-1", AvailableQuantity: 7}, nil).Once()
	repo.On("SaveCart", ctx, mock.AnythingOfType("*models.Cart"), 15*time.Minute).Return(nil).Once()

	cart, err := service.AddCartItem(ctx, 1, "product-1", 1)

	assert.NoError(t, err)
	assert.Equal(t, int32(3), cart.Items[0].Quantity)
	repo.AssertExpectations(t)
}

func TestCartService_DoesNotMutateWhenAccountLockIsBusy(t *testing.T) {
	granted := false
	repo := &MockRepository{cartLockGranted: &granted}
	service := internal.NewCartService(repo, new(MockInventoryClient), newStubAsyncProducer())

	cart, err := service.AddCartItem(context.Background(), 1, "product-1", 1)

	assert.ErrorIs(t, err, internal.ErrCartLocked)
	assert.Nil(t, cart)
	repo.AssertNotCalled(t, "GetCart", mock.Anything, mock.Anything)
}

func TestCartService_ReleasesNewReservationWhenCartSaveFails(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())
	reservation := &inventorymodels.StockReservation{ReservationID: "r-new", ProductID: "product-1", Quantity: 1}
	repo.On("GetCart", ctx, uint64(1)).Return((*models.Cart)(nil), internal.ErrCartNotFound).Once()
	inv.On("ReserveStock", ctx, "", uint64(1), "product-1", int32(1), 15*time.Minute, "cart").Return(reservation, &inventorymodels.Availability{}, nil).Once()
	repo.On("SaveCart", ctx, mock.AnythingOfType("*models.Cart"), 15*time.Minute).Return(errors.New("redis unavailable")).Once()
	inv.On("ReleaseReservation", mock.MatchedBy(func(got context.Context) bool { return got.Err() == nil }), "r-new").Return(&inventorymodels.StockReservation{ReservationID: "r-new"}, &inventorymodels.Availability{}, nil).Once()

	cart, err := service.AddCartItem(ctx, 1, "product-1", 1)

	assert.Nil(t, cart)
	assert.EqualError(t, err, "redis unavailable")
	inv.AssertExpectations(t)
}

func TestCartService_CompensatesReservationAfterRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := new(MockRepository)
	inv := new(MockInventoryClient)
	service := internal.NewCartService(repo, inv, newStubAsyncProducer())
	reservation := &inventorymodels.StockReservation{ReservationID: "r-cancelled", ProductID: "product-1", Quantity: 1}
	repo.On("GetCart", ctx, uint64(1)).Return((*models.Cart)(nil), internal.ErrCartNotFound).Once()
	inv.On("ReserveStock", ctx, "", uint64(1), "product-1", int32(1), 15*time.Minute, "cart").Run(func(mock.Arguments) { cancel() }).Return(reservation, &inventorymodels.Availability{}, nil).Once()
	repo.On("SaveCart", ctx, mock.AnythingOfType("*models.Cart"), 15*time.Minute).Return(context.Canceled).Once()
	inv.On("ReleaseReservation", mock.MatchedBy(func(got context.Context) bool { return got.Err() == nil }), "r-cancelled").Return(
		&inventorymodels.StockReservation{ReservationID: "r-cancelled"}, &inventorymodels.Availability{}, nil).Once()

	cart, err := service.AddCartItem(ctx, 1, "product-1", 1)

	assert.Nil(t, cart)
	assert.ErrorIs(t, err, context.Canceled)
	inv.AssertExpectations(t)
}

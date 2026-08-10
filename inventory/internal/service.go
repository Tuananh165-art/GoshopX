package internal

import (
	"context"
	"errors"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/inventory/config"
	"github.com/Tuananh165art/GoshopX/inventory/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
	"github.com/google/uuid"
)

var (
	ErrInvalidQuantity   = errors.New("invalid quantity")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Service interface {
	UpsertStock(ctx context.Context, productID string, quantity, reorderLevel int32) (*models.Stock, *models.Availability, error)
	GetAvailability(ctx context.Context, productID string) (*models.Availability, error)
	ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, ttl time.Duration, source string) (*models.StockReservation, *models.Availability, error)
	ReleaseReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error)
	CommitReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error)
	ListLowStock(ctx context.Context, limit uint64) ([]*models.Stock, error)
	CleanupExpiredReservations(ctx context.Context) (int64, error)
	AdjustStock(ctx context.Context, productID string, delta, reorderLevel int32, reason string) (*models.Stock, *models.Availability, error)
	ListReservations(ctx context.Context, status string, skip, take uint64) ([]*models.StockReservation, error)
	GetProducer() sarama.AsyncProducer
}

func (service *inventoryService) AdjustStock(ctx context.Context, productID string, delta, reorderLevel int32, reason string) (*models.Stock, *models.Availability, error) {
	if reason == "" {
		return nil, nil, errors.New("adjustment reason is required")
	}
	stock, err := service.repository.AdjustStock(ctx, productID, delta, reorderLevel)
	if err != nil {
		return nil, nil, err
	}
	availability, err := service.repository.GetAvailability(ctx, productID)
	if err != nil {
		return nil, nil, err
	}
	service.publishInventoryEvent("inventory.adjusted", 0, productID, models.EventData{ProductID: productID, Quantity: delta, ReorderLevel: stock.ReorderLevel, AvailableQty: availability.AvailableQuantity, ReservedQty: availability.ReservedQuantity, Status: "adjusted:" + reason})
	return stock, availability, nil
}
func (service *inventoryService) ListReservations(ctx context.Context, status string, skip, take uint64) ([]*models.StockReservation, error) {
	return service.repository.ListReservations(ctx, status, skip, take)
}

type inventoryService struct {
	repository Repository
	producer   sarama.AsyncProducer
}

func NewInventoryService(repository Repository, producer sarama.AsyncProducer) Service {
	return &inventoryService{repository: repository, producer: producer}
}

func (service *inventoryService) GetProducer() sarama.AsyncProducer {
	return service.producer
}

func (service *inventoryService) UpsertStock(ctx context.Context, productID string, quantity, reorderLevel int32) (*models.Stock, *models.Availability, error) {
	stock, err := service.repository.UpsertStock(ctx, productID, quantity, reorderLevel)
	if err != nil {
		return nil, nil, err
	}

	availability, err := service.repository.GetAvailability(ctx, productID)
	if err != nil {
		return nil, nil, err
	}

	service.publishInventoryEvent("inventory.stock_upserted", 0, productID, models.EventData{
		ProductID:    productID,
		Quantity:     quantity,
		ReorderLevel: reorderLevel,
		AvailableQty: availability.AvailableQuantity,
		ReservedQty:  availability.ReservedQuantity,
		Status:       "upserted",
	})

	return stock, availability, nil
}

func (service *inventoryService) GetAvailability(ctx context.Context, productID string) (*models.Availability, error) {
	return service.repository.GetAvailability(ctx, productID)
}

func (service *inventoryService) ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, ttl time.Duration, source string) (*models.StockReservation, *models.Availability, error) {
	if quantity <= 0 {
		return nil, nil, ErrInvalidQuantity
	}
	if reservationID == "" {
		reservationID = uuid.NewString()
	}

	reservation, availability, err := service.repository.ReserveStock(ctx, reservationID, accountID, productID, quantity, time.Now().UTC().Add(ttl), source)
	if err != nil {
		return nil, nil, err
	}

	service.publishInventoryEvent("inventory.reserved", accountID, reservation.ReservationID, models.EventData{
		ProductID:      reservation.ProductID,
		ReservationID:  reservation.ReservationID,
		Quantity:       reservation.Quantity,
		ReorderLevel:   availability.ReorderLevel,
		AvailableQty:   availability.AvailableQuantity,
		ReservedQty:    availability.ReservedQuantity,
		ReservationFor: reservation.Source,
		Status:         string(reservation.Status),
	})

	return reservation, availability, nil
}

func (service *inventoryService) ReleaseReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	reservation, availability, err := service.repository.ReleaseReservation(ctx, reservationID)
	if err != nil {
		return nil, nil, err
	}
	service.publishInventoryEvent("inventory.released", reservation.AccountID, reservation.ReservationID, models.EventData{
		ProductID:      reservation.ProductID,
		ReservationID:  reservation.ReservationID,
		Quantity:       reservation.Quantity,
		ReorderLevel:   availability.ReorderLevel,
		AvailableQty:   availability.AvailableQuantity,
		ReservedQty:    availability.ReservedQuantity,
		ReservationFor: reservation.Source,
		Status:         string(reservation.Status),
	})
	return reservation, availability, nil
}

func (service *inventoryService) CommitReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	reservation, availability, err := service.repository.CommitReservation(ctx, reservationID)
	if err != nil {
		return nil, nil, err
	}
	service.publishInventoryEvent("inventory.committed", reservation.AccountID, reservation.ReservationID, models.EventData{
		ProductID:      reservation.ProductID,
		ReservationID:  reservation.ReservationID,
		Quantity:       reservation.Quantity,
		ReorderLevel:   availability.ReorderLevel,
		AvailableQty:   availability.AvailableQuantity,
		ReservedQty:    availability.ReservedQuantity,
		ReservationFor: reservation.Source,
		Status:         string(reservation.Status),
	})
	return reservation, availability, nil
}

func (service *inventoryService) ListLowStock(ctx context.Context, limit uint64) ([]*models.Stock, error) {
	return service.repository.ListLowStock(ctx, limit)
}

func (service *inventoryService) CleanupExpiredReservations(ctx context.Context) (int64, error) {
	return service.repository.CleanupExpiredReservations(ctx)
}

func (service *inventoryService) publishInventoryEvent(eventType string, accountID uint64, eventKey string, data models.EventData) {
	go func() {
		_ = kafka.SendMessage(service, sharedevents.New(eventType, accountID, eventKey, data), config.InventoryTopic)
	}()
}

package internal

import (
	"context"
	"errors"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/cart/config"
	"github.com/Tuananh165art/GoshopX/cart/models"
	inventorymodels "github.com/Tuananh165art/GoshopX/inventory/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
)

var ErrInvalidCartQuantity = errors.New("invalid cart quantity")

type InventoryClient interface {
	ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, ttl time.Duration, source string) (*inventorymodels.StockReservation, *inventorymodels.Availability, error)
	ReleaseReservation(ctx context.Context, reservationID string) (*inventorymodels.StockReservation, *inventorymodels.Availability, error)
}

type Service interface {
	GetCart(ctx context.Context, accountID uint64) (*models.Cart, error)
	UpsertCartItem(ctx context.Context, accountID uint64, productID string, quantity int32) (*models.Cart, error)
	RemoveCartItem(ctx context.Context, accountID uint64, productID string) (*models.Cart, error)
	ClearCart(ctx context.Context, accountID uint64) error
	PrepareCheckout(ctx context.Context, accountID uint64) (*models.Cart, error)
	GetProducer() sarama.AsyncProducer
}

type cartService struct {
	repository      Repository
	inventoryClient InventoryClient
	producer        sarama.AsyncProducer
	ttl             time.Duration
}

func NewCartService(repository Repository, inventoryClient InventoryClient, producer sarama.AsyncProducer) Service {
	return &cartService{
		repository:      repository,
		inventoryClient: inventoryClient,
		producer:        producer,
		ttl:             time.Duration(config.ReservationTTL) * time.Second,
	}
}

func (service *cartService) GetProducer() sarama.AsyncProducer {
	return service.producer
}

func (service *cartService) GetCart(ctx context.Context, accountID uint64) (*models.Cart, error) {
	cart, err := service.repository.GetCart(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrCartNotFound) {
			return &models.Cart{AccountID: accountID, Items: []models.CartItem{}}, nil
		}
		return nil, err
	}
	return service.filterExpired(cart), nil
}

func (service *cartService) UpsertCartItem(ctx context.Context, accountID uint64, productID string, quantity int32) (*models.Cart, error) {
	if quantity <= 0 {
		return nil, ErrInvalidCartQuantity
	}

	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}

	itemIndex := findItem(cart.Items, productID)
	reservationID := ""
	if itemIndex >= 0 {
		reservationID = cart.Items[itemIndex].ReservationID
	}

	reservation, _, err := service.inventoryClient.ReserveStock(ctx, reservationID, accountID, productID, quantity, service.ttl, "cart")
	if err != nil {
		return nil, err
	}

	item := models.CartItem{
		ProductID:     productID,
		Quantity:      quantity,
		ReservationID: reservation.ReservationID,
		ReservedUntil: reservation.ExpiresAt,
	}

	if itemIndex >= 0 {
		cart.Items[itemIndex] = item
	} else {
		cart.Items = append(cart.Items, item)
	}

	cart.ExpiresAt = reservation.ExpiresAt
	if err := service.repository.SaveCart(ctx, cart, service.ttl); err != nil {
		return nil, err
	}

	service.publishEvent(accountID, "cart.item_upserted", models.EventData{
		ProductID:     productID,
		Quantity:      quantity,
		ReservationID: reservation.ReservationID,
		CartSize:      len(cart.Items),
		Action:        "upserted",
	})

	return cart, nil
}

func (service *cartService) RemoveCartItem(ctx context.Context, accountID uint64, productID string) (*models.Cart, error) {
	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}

	itemIndex := findItem(cart.Items, productID)
	if itemIndex < 0 {
		return cart, nil
	}

	item := cart.Items[itemIndex]
	if item.ReservationID != "" {
		if _, _, err := service.inventoryClient.ReleaseReservation(ctx, item.ReservationID); err != nil {
			return nil, err
		}
	}

	cart.Items = append(cart.Items[:itemIndex], cart.Items[itemIndex+1:]...)
	if len(cart.Items) == 0 {
		if err := service.repository.DeleteCart(ctx, accountID); err != nil {
			return nil, err
		}
	} else {
		cart.ExpiresAt = time.Now().UTC().Add(service.ttl)
		if err := service.repository.SaveCart(ctx, cart, service.ttl); err != nil {
			return nil, err
		}
	}

	service.publishEvent(accountID, "cart.item_removed", models.EventData{
		ProductID:     productID,
		Quantity:      item.Quantity,
		ReservationID: item.ReservationID,
		CartSize:      len(cart.Items),
		Action:        "removed",
	})

	return cart, nil
}

func (service *cartService) ClearCart(ctx context.Context, accountID uint64) error {
	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrCartNotFound) {
			return nil
		}
		return err
	}

	var reservationIDs []string
	for _, item := range cart.Items {
		if item.ReservationID == "" {
			continue
		}
		reservationIDs = append(reservationIDs, item.ReservationID)
		if _, _, err := service.inventoryClient.ReleaseReservation(ctx, item.ReservationID); err != nil {
			return err
		}
	}

	if err := service.repository.DeleteCart(ctx, accountID); err != nil {
		return err
	}

	service.publishEvent(accountID, "cart.cleared", models.EventData{
		ReservationIDs: reservationIDs,
		CartSize:       0,
		Action:         "cleared",
	})
	return nil
}

func (service *cartService) PrepareCheckout(ctx context.Context, accountID uint64) (*models.Cart, error) {
	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}

	filtered := service.filterExpired(cart)
	if len(filtered.Items) == 0 {
		return filtered, nil
	}

	service.publishEvent(accountID, "cart.checkout_prepared", models.EventData{
		ReservationIDs: reservationIDs(filtered.Items),
		CartSize:       len(filtered.Items),
		Action:         "prepared",
	})
	return filtered, nil
}

func (service *cartService) filterExpired(cart *models.Cart) *models.Cart {
	now := time.Now().UTC()
	filtered := &models.Cart{
		AccountID: cart.AccountID,
		Items:     make([]models.CartItem, 0, len(cart.Items)),
		ExpiresAt: cart.ExpiresAt,
	}
	for _, item := range cart.Items {
		if item.ReservedUntil.After(now) {
			filtered.Items = append(filtered.Items, item)
		}
	}
	return filtered
}

func (service *cartService) publishEvent(accountID uint64, eventType string, data models.EventData) {
	go func() {
		_ = kafka.SendMessage(service, sharedevents.New(eventType, accountID, eventType, data), config.CartTopic)
	}()
}

func findItem(items []models.CartItem, productID string) int {
	for i, item := range items {
		if item.ProductID == productID {
			return i
		}
	}
	return -1
}

func reservationIDs(items []models.CartItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item.ReservationID != "" {
			result = append(result, item.ReservationID)
		}
	}
	return result
}


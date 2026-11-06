package internal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"math"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/cart/config"
	"github.com/Tuananh165art/GoshopX/cart/models"
	inventorymodels "github.com/Tuananh165art/GoshopX/inventory/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
	"github.com/google/uuid"
)

var ErrInvalidCartQuantity = errors.New("invalid cart quantity")
var ErrCartVersionConflict = errors.New("cart changed; refresh and review the proposal")
var ErrCartProposalExpired = errors.New("cart proposal expired")
var ErrCartProposalMismatch = errors.New("idempotency key was already used for another proposal")

type InventoryClient interface {
	ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, ttl time.Duration, source string) (*inventorymodels.StockReservation, *inventorymodels.Availability, error)
	ReleaseReservation(ctx context.Context, reservationID string) (*inventorymodels.StockReservation, *inventorymodels.Availability, error)
}

type Service interface {
	GetCart(ctx context.Context, accountID uint64) (*models.Cart, error)
	UpsertCartItem(ctx context.Context, accountID uint64, productID string, quantity int32) (*models.Cart, error)
	AddCartItem(ctx context.Context, accountID uint64, productID string, quantity int32) (*models.Cart, error)
	RemoveCartItem(ctx context.Context, accountID uint64, productID string) (*models.Cart, error)
	ClearCart(ctx context.Context, accountID uint64) error
	CompleteCheckout(ctx context.Context, accountID uint64) error
	PrepareCheckout(ctx context.Context, accountID uint64) (*models.Cart, error)
	PrepareCheckoutAtVersion(ctx context.Context, accountID, expectedVersion uint64) (*models.Cart, error)
	CreateCartProposal(ctx context.Context, accountID, expectedVersion uint64, idempotencyKey string, items []models.ProposalItem) (*models.CartProposal, error)
	ConfirmCartProposal(ctx context.Context, accountID uint64, proposalID string) (*models.Cart, error)
	GetProducer() sarama.AsyncProducer
}

type cartService struct {
	repository      Repository
	inventoryClient InventoryClient
	producer        sarama.AsyncProducer
	ttl             time.Duration
}

const cartMutationLockTTL = 2 * time.Minute
const cartProposalTTL = 10 * time.Minute
const maxCartProposalItems = 20

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
	return service.upsertCartItem(ctx, accountID, productID, quantity, false)
}

func (service *cartService) AddCartItem(ctx context.Context, accountID uint64, productID string, quantity int32) (*models.Cart, error) {
	return service.upsertCartItem(ctx, accountID, productID, quantity, true)
}

func (service *cartService) upsertCartItem(ctx context.Context, accountID uint64, productID string, quantity int32, increment bool) (*models.Cart, error) {
	unlock, err := service.lockCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if quantity <= 0 {
		return nil, ErrInvalidCartQuantity
	}

	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := incrementCartVersion(cart); err != nil {
		return nil, err
	}

	itemIndex := findItem(cart.Items, productID)
	if increment && itemIndex >= 0 {
		quantity += cart.Items[itemIndex].Quantity
	}
	reservationID := ""
	var previousItem models.CartItem
	if itemIndex >= 0 {
		previousItem = cart.Items[itemIndex]
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
		// Restore the prior reservation quantity if persistence fails after Inventory accepted it.
		compensationCtx, cancelCompensation := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelCompensation()
		var compensationErr error
		if itemIndex >= 0 {
			_, _, compensationErr = service.inventoryClient.ReserveStock(compensationCtx, previousItem.ReservationID, accountID, productID, previousItem.Quantity, service.ttl, "cart")
		} else {
			_, _, compensationErr = service.inventoryClient.ReleaseReservation(compensationCtx, reservation.ReservationID)
		}
		if compensationErr != nil {
			log.Printf("cart reservation compensation failed for account %d product %s: %v", accountID, productID, compensationErr)
		}
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
	unlock, err := service.lockCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}

	itemIndex := findItem(cart.Items, productID)
	if itemIndex < 0 {
		return cart, nil
	}

	item := cart.Items[itemIndex]
	cart.Items = append(cart.Items[:itemIndex], cart.Items[itemIndex+1:]...)
	if err := incrementCartVersion(cart); err != nil {
		return nil, err
	}
	cart.ExpiresAt = time.Now().UTC().Add(service.ttl)
	if err := service.repository.SaveCart(ctx, cart, service.ttl); err != nil {
		return nil, err
	}
	if item.ReservationID != "" {
		if _, _, err := service.inventoryClient.ReleaseReservation(ctx, item.ReservationID); err != nil {
			log.Printf("cart reservation release deferred for account %d product %s: %v", accountID, productID, err)
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
	unlock, err := service.lockCart(ctx, accountID)
	if err != nil {
		return err
	}
	defer unlock()

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
	}

	if err := incrementCartVersion(cart); err != nil {
		return err
	}
	cart.Items = []models.CartItem{}
	cart.ExpiresAt = time.Now().UTC().Add(service.ttl)
	if err := service.repository.SaveCart(ctx, cart, service.ttl); err != nil {
		return err
	}
	for _, reservationID := range reservationIDs {
		if _, _, err := service.inventoryClient.ReleaseReservation(ctx, reservationID); err != nil {
			log.Printf("cart reservation release deferred for account %d reservation %s: %v", accountID, reservationID, err)
		}
	}

	service.publishEvent(accountID, "cart.cleared", models.EventData{
		ReservationIDs: reservationIDs,
		CartSize:       0,
		Action:         "cleared",
	})
	return nil
}

func (service *cartService) CompleteCheckout(ctx context.Context, accountID uint64) error {
	unlock, err := service.lockCart(ctx, accountID)
	if err != nil {
		return err
	}
	defer unlock()

	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return err
	}
	if err := incrementCartVersion(cart); err != nil {
		return err
	}
	cart.Items = []models.CartItem{}
	cart.ExpiresAt = time.Now().UTC().Add(service.ttl)
	if err := service.repository.SaveCart(ctx, cart, service.ttl); err != nil {
		return err
	}
	service.publishEvent(accountID, "cart.checkout_completed", models.EventData{CartSize: 0, Action: "completed"})
	return nil
}

func (service *cartService) lockCart(ctx context.Context, accountID uint64) (func(), error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(bytes[:])
	// shortcut: the lease is fixed because Cart operations are bounded by the GraphQL deadlines; add lease renewal if that changes.
	locked, err := service.repository.AcquireCartLock(ctx, accountID, token, cartMutationLockTTL)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrCartLocked
	}
	return func() { _ = service.repository.ReleaseCartLock(context.Background(), accountID, token) }, nil
}

func (service *cartService) PrepareCheckout(ctx context.Context, accountID uint64) (*models.Cart, error) {
	return service.PrepareCheckoutAtVersion(ctx, accountID, 0)
}

func (service *cartService) PrepareCheckoutAtVersion(ctx context.Context, accountID, expectedVersion uint64) (*models.Cart, error) {
	unlock, err := service.lockCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if expectedVersion != 0 && cart.Version != expectedVersion {
		return nil, ErrCartVersionConflict
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

func (service *cartService) CreateCartProposal(ctx context.Context, accountID, expectedVersion uint64, idempotencyKey string, items []models.ProposalItem) (*models.CartProposal, error) {
	if accountID == 0 || len(idempotencyKey) == 0 || len(idempotencyKey) > 128 || len(items) == 0 || len(items) > maxCartProposalItems {
		return nil, ErrInvalidCartQuantity
	}
	copyItems := append([]models.ProposalItem(nil), items...)
	seen := make(map[string]struct{}, len(copyItems))
	for _, item := range copyItems {
		if item.ProductID == "" || len(item.ProductID) > 128 || item.Quantity <= 0 || item.Quantity > 100 {
			return nil, ErrInvalidCartQuantity
		}
		if _, exists := seen[item.ProductID]; exists {
			return nil, ErrInvalidCartQuantity
		}
		seen[item.ProductID] = struct{}{}
	}
	unlock, err := service.lockCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if existing, err := service.repository.GetCartProposal(ctx, accountID, idempotencyKey); err == nil {
		if existing.BaseVersion != expectedVersion || !sameProposalItems(existing.Items, copyItems) {
			return nil, ErrCartProposalMismatch
		}
		return existing, nil
	} else if !errors.Is(err, ErrCartNotFound) {
		return nil, err
	}

	cart, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if cart.Version != expectedVersion {
		return nil, ErrCartVersionConflict
	}
	proposal := &models.CartProposal{
		ID: uuid.NewString(), IdempotencyKey: idempotencyKey, AccountID: accountID,
		BaseVersion: expectedVersion, Items: copyItems, ExpiresAt: time.Now().UTC().Add(cartProposalTTL),
	}
	if err := service.repository.SaveCartProposal(ctx, proposal, cartProposalTTL); err != nil {
		return nil, err
	}
	return proposal, nil
}

func (service *cartService) ConfirmCartProposal(ctx context.Context, accountID uint64, proposalID string) (*models.Cart, error) {
	proposal, err := service.repository.GetCartProposalByID(ctx, accountID, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal.AccountID != accountID || proposal.ID != proposalID {
		return nil, ErrCartNotFound
	}
	if proposal.Result != nil {
		return proposal.Result, nil
	}
	if !proposal.ExpiresAt.After(time.Now().UTC()) {
		return nil, ErrCartProposalExpired
	}
	unlock, err := service.lockCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer unlock()

	current, err := service.GetCart(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if current.Version != proposal.BaseVersion {
		return nil, ErrCartVersionConflict
	}
	if err := incrementCartVersion(current); err != nil {
		return nil, err
	}
	type reservationChange struct {
		id               string
		productID        string
		previousQuantity int32
	}
	changes := make([]reservationChange, 0, len(proposal.Items))
	rollback := func() {
		compensationCtx, cancelCompensation := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelCompensation()
		for i := len(changes) - 1; i >= 0; i-- {
			change := changes[i]
			if change.previousQuantity > 0 {
				_, _, _ = service.inventoryClient.ReserveStock(compensationCtx, change.id, accountID, change.productID, change.previousQuantity, service.ttl, "cart")
			} else {
				_, _, _ = service.inventoryClient.ReleaseReservation(compensationCtx, change.id)
			}
		}
	}
	result := &models.Cart{AccountID: accountID, Version: current.Version, Items: append([]models.CartItem(nil), current.Items...), ExpiresAt: time.Now().UTC().Add(service.ttl)}
	for _, item := range proposal.Items {
		index := findItem(result.Items, item.ProductID)
		reservationID := proposalReservationID(proposal.ID, item.ProductID)
		quantity := item.Quantity
		previousQuantity := int32(0)
		if index >= 0 {
			previous := result.Items[index]
			if previous.Quantity > 100-item.Quantity {
				rollback()
				return nil, ErrInvalidCartQuantity
			}
			quantity += previous.Quantity
			reservationID = previous.ReservationID
			if previous.ReservationID != "" {
				previousQuantity = previous.Quantity
			}
		}
		reservation, _, reserveErr := service.inventoryClient.ReserveStock(ctx, reservationID, accountID, item.ProductID, quantity, service.ttl, "cart")
		if reserveErr != nil {
			rollback()
			return nil, reserveErr
		}
		changes = append(changes, reservationChange{id: reservation.ReservationID, productID: item.ProductID, previousQuantity: previousQuantity})
		updated := models.CartItem{ProductID: item.ProductID, Quantity: quantity, ReservationID: reservation.ReservationID, ReservedUntil: reservation.ExpiresAt}
		if index >= 0 {
			result.Items[index] = updated
		} else {
			result.Items = append(result.Items, updated)
		}
	}
	proposal.Result = result
	if err := service.repository.SaveCartAndProposal(ctx, result, proposal, service.ttl); err != nil {
		rollback()
		return nil, err
	}
	service.publishEvent(accountID, "cart.proposal_confirmed", models.EventData{ReservationIDs: reservationIDs(result.Items), CartSize: len(result.Items), Action: "proposal_confirmed"})
	return result, nil
}

func sameProposalItems(left, right []models.ProposalItem) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func proposalReservationID(proposalID, productID string) string {
	digest := sha256.Sum256([]byte(proposalID + ":" + productID))
	return "cart-proposal-" + hex.EncodeToString(digest[:])
}

func (service *cartService) filterExpired(cart *models.Cart) *models.Cart {
	now := time.Now().UTC()
	filtered := &models.Cart{
		AccountID: cart.AccountID,
		Version:   cart.Version,
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

func incrementCartVersion(cart *models.Cart) error {
	if cart.Version == math.MaxUint64 {
		return errors.New("cart version exhausted")
	}
	cart.Version++
	return nil
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

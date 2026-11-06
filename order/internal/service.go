package internal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/order/config"
	"github.com/Tuananh165art/GoshopX/order/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
)

var ErrInvalidOrderTotal = errors.New("invalid order total")

type Service interface {
	PostOrder(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct, cashOnDelivery bool) (*models.Order, error)
	PostOrderWithCoupon(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct, cashOnDelivery bool, couponCode string) (*models.Order, error)
	PostOrderWithQuote(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct, cashOnDelivery bool, quoteID string, cartVersion uint64) (*models.Order, error)
	PreviewCheckout(ctx context.Context, accountID uint64, subtotalVND int64, products []*models.OrderedProduct, couponCode string, cartVersion uint64) (*models.CheckoutQuote, error)
	GetOrdersForAccount(ctx context.Context, accountID uint64) ([]*models.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, status string) error
	ListOrders(ctx context.Context, status, paymentStatus string, accountID, skip, take uint64) ([]*models.Order, error)
	GetOrder(ctx context.Context, orderID uint64) (*models.Order, error)
	CancelOrder(ctx context.Context, orderID uint64, reason string) (*models.Order, error)
	GetProducer() sarama.AsyncProducer
}

type orderService struct {
	repository Repository
	producer   sarama.AsyncProducer
}

func NewOrderService(repository Repository, producer sarama.AsyncProducer) Service {
	return &orderService{repository, producer}
}

func (service orderService) GetProducer() sarama.AsyncProducer {
	return service.producer
}

func (service orderService) PostOrder(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct, cashOnDelivery bool) (*models.Order, error) {
	return service.postOrder(ctx, accountID, totalPrice, products, cashOnDelivery, "")
}

func (service orderService) PostOrderWithCoupon(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct, cashOnDelivery bool, couponCode string) (*models.Order, error) {
	return service.postOrder(ctx, accountID, totalPrice, products, cashOnDelivery, couponCode)
}

func (service orderService) PreviewCheckout(ctx context.Context, accountID uint64, subtotalVND int64, products []*models.OrderedProduct, couponCode string, cartVersion uint64) (*models.CheckoutQuote, error) {
	if accountID == 0 || subtotalVND < 0 || cartVersion == 0 || len(products) == 0 || len(products) > 100 {
		return nil, ErrInvalidOrderTotal
	}
	code := strings.ToUpper(strings.TrimSpace(couponCode))
	discount, err := service.repository.PreviewCoupon(ctx, accountID, subtotalVND, code)
	if err != nil {
		return nil, err
	}
	quoteIDBytes := make([]byte, 24)
	if _, err := rand.Read(quoteIDBytes); err != nil {
		return nil, err
	}
	quote := &models.CheckoutQuote{ID: hex.EncodeToString(quoteIDBytes), SubtotalVND: subtotalVND, DiscountVND: discount, TotalPriceVND: subtotalVND - discount, CouponCode: code, ExpiresAt: time.Now().UTC().Add(10 * time.Minute), CartVersion: cartVersion}
	record := &models.CheckoutQuoteRecord{ID: quote.ID, AccountID: accountID, ItemsHash: models.CheckoutItemsHash(products), SubtotalVND: quote.SubtotalVND, DiscountVND: quote.DiscountVND, TotalPriceVND: quote.TotalPriceVND, CouponCode: quote.CouponCode, ExpiresAt: quote.ExpiresAt, CartVersion: quote.CartVersion}
	if err := service.repository.CreateCheckoutQuote(ctx, record); err != nil {
		return nil, err
	}
	return quote, nil
}

func (service orderService) PostOrderWithQuote(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct, cashOnDelivery bool, quoteID string, cartVersion uint64) (*models.Order, error) {
	if accountID == 0 || quoteID == "" || cartVersion == 0 || len(products) == 0 || len(products) > 100 {
		return nil, ErrInvalidCheckoutQuote
	}
	if math.IsNaN(totalPrice) || math.IsInf(totalPrice, 0) || totalPrice < 0 || totalPrice >= float64(math.MaxInt64) {
		return nil, ErrInvalidOrderTotal
	}
	subtotalVND := int64(math.Round(totalPrice))
	order := &models.Order{AccountID: accountID, TotalPrice: totalPrice, SubtotalVND: subtotalVND, TotalPriceVND: subtotalVND, Products: products, CreatedAt: time.Now().UTC(), Status: "pending", PaymentStatus: "pending"}
	created, err := service.repository.PutOrderWithQuote(ctx, order, quoteID, models.CheckoutItemsHash(products), cartVersion)
	if err != nil {
		return nil, err
	}
	if !created {
		return order, nil
	}
	go func() {
		for _, product := range products {
			if err := kafka.SendMessageToRecommender(service, models.Event{Type: "purchase", EventData: models.EventData{AccountId: int(accountID), ProductId: product.ID}}, "interaction_events"); err != nil {
				log.Println("Failed to send event to recommendation service:", err)
			}
		}
	}()
	go func() {
		eventType, eventKey := orderCreatedEvent(cashOnDelivery)
		if sendErr := kafka.SendMessage(service, sharedevents.New(eventType, accountID, eventKey, orderEventData(order)), config.OrderEventsTopic); sendErr != nil {
			log.Println("Failed to send order event:", sendErr)
		}
	}()
	return order, nil
}

func (service orderService) postOrder(ctx context.Context, accountID uint64, totalPrice float64, products []*models.OrderedProduct, cashOnDelivery bool, couponCode string) (*models.Order, error) {
	if math.IsNaN(totalPrice) || math.IsInf(totalPrice, 0) || totalPrice < 0 || totalPrice >= float64(math.MaxInt64) {
		return nil, ErrInvalidOrderTotal
	}
	subtotalVND := int64(math.Round(totalPrice))
	order := models.Order{
		AccountID:     accountID,
		TotalPrice:    totalPrice,
		SubtotalVND:   subtotalVND,
		TotalPriceVND: subtotalVND,
		Products:      products,
		CreatedAt:     time.Now().UTC(),
		Status:        "pending",
		PaymentStatus: "pending",
	}
	var err error
	if couponCode == "" {
		err = service.repository.PutOrder(ctx, &order)
	} else {
		err = service.repository.PutOrderWithCoupon(ctx, &order, couponCode)
	}
	if err != nil {
		return nil, err
	}

	// Send to recommendation service
	go func() {
		if err != nil {
			log.Println("Failed to convert account ID to int:", err)
			return
		}
		for _, product := range products {
			err = kafka.SendMessageToRecommender(service, models.Event{
				Type: "purchase",
				EventData: models.EventData{
					AccountId: int(accountID),
					ProductId: product.ID,
				},
			}, "interaction_events")
			if err != nil {
				log.Println("Failed to send event to recommendation service:", err)
			}
		}
	}()

	go func() {
		eventType, eventKey := orderCreatedEvent(cashOnDelivery)
		if sendErr := kafka.SendMessage(service, sharedevents.New(eventType, accountID, eventKey, orderEventData(&order)), config.OrderEventsTopic); sendErr != nil {
			log.Println("Failed to send order event:", sendErr)
		}
	}()

	return &order, nil
}

func orderCreatedEvent(cashOnDelivery bool) (string, string) {
	if cashOnDelivery {
		return "order.cod_created", "order-cod-created"
	}
	return "order.created", "order-created"
}

func (service orderService) ListOrders(ctx context.Context, status, paymentStatus string, accountID, skip, take uint64) ([]*models.Order, error) {
	return service.repository.ListOrders(ctx, status, paymentStatus, accountID, skip, take)
}
func (service orderService) GetOrder(ctx context.Context, orderID uint64) (*models.Order, error) {
	return service.repository.GetOrderByID(ctx, orderID)
}
func (service orderService) CancelOrder(ctx context.Context, orderID uint64, reason string) (*models.Order, error) {
	if reason == "" {
		return nil, errors.New("cancellation reason is required")
	}
	order, err := service.repository.CancelOrder(ctx, orderID, reason)
	if err != nil {
		return nil, err
	}
	go func() {
		_ = kafka.SendMessage(service, sharedevents.New("order.cancelled", order.AccountID, fmt.Sprintf("order-%d", order.ID), map[string]any{"order_id": order.ID, "reason": reason}), config.OrderEventsTopic)
	}()
	return order, nil
}

func (service orderService) GetOrdersForAccount(ctx context.Context, accountID uint64) ([]*models.Order, error) {
	return service.repository.GetOrdersForAccount(ctx, accountID)
}

func (service orderService) UpdateOrderPaymentStatus(ctx context.Context, orderId uint64, paymnetStatus string) error {
	if err := service.repository.UpdateOrderPaymentStatus(ctx, orderId, paymnetStatus); err != nil {
		return err
	}
	go func() {
		order, err := service.repository.GetOrderByID(context.Background(), orderId)
		if err != nil {
			log.Println("Failed to load order for status event:", err)
			return
		}
		data := orderEventData(order)
		data["status"] = order.Status
		data["payment_status"] = paymnetStatus
		eventType := "order.payment_status_updated"
		if err := kafka.SendMessage(service, sharedevents.New(eventType, order.AccountID, fmt.Sprintf("order-%d-status-%s", orderId, paymnetStatus), data), config.OrderEventsTopic); err != nil {
			log.Println("Failed to send order status event:", err)
		}
	}()
	return nil
}

func orderEventData(order *models.Order) map[string]any {
	products := make([]map[string]any, 0, len(order.Products))
	for _, product := range order.Products {
		products = append(products, map[string]any{
			"id":          product.ID,
			"name":        product.Name,
			"description": product.Description,
			"price":       product.Price,
			"quantity":    product.Quantity,
		})
	}
	return map[string]any{
		"order_id":       order.ID,
		"account_id":     order.AccountID,
		"total_price":    order.TotalPrice,
		"currency":       "USD",
		"status":         order.Status,
		"payment_status": order.PaymentStatus,
		"products":       products,
	}
}

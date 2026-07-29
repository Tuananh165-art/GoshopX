package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Tuananh165art/GoshopX/notification/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
)

type Service interface {
	ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error)
	CountUnread(ctx context.Context, accountID uint64) (int64, error)
	MarkRead(ctx context.Context, accountID uint64, notificationID uint64) (bool, error)
	MarkAllRead(ctx context.Context, accountID uint64) (int64, error)
	CreateFromEvent(ctx context.Context, event *sharedevents.Envelope) error
}

type notificationService struct {
	repository Repository
}

func NewNotificationService(repository Repository) Service {
	return &notificationService{repository: repository}
}

func (service *notificationService) ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error) {
	return service.repository.ListNotifications(ctx, accountID, skip, take)
}

func (service *notificationService) CountUnread(ctx context.Context, accountID uint64) (int64, error) {
	return service.repository.CountUnread(ctx, accountID)
}

func (service *notificationService) MarkRead(ctx context.Context, accountID uint64, notificationID uint64) (bool, error) {
	return service.repository.MarkRead(ctx, accountID, notificationID)
}

func (service *notificationService) MarkAllRead(ctx context.Context, accountID uint64) (int64, error) {
	return service.repository.MarkAllRead(ctx, accountID)
}

func (service *notificationService) CreateFromEvent(ctx context.Context, event *sharedevents.Envelope) error {
	if event == nil || event.AccountID == 0 {
		return nil
	}

	title, message := notificationCopy(event.EventType)
	if title == "" && message == "" {
		return nil
	}

	body, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	notification := &models.Notification{
		AccountID:    event.AccountID,
		EventID:      event.EventID,
		EventType:    event.EventType,
		Title:        title,
		Message:      message,
		MetadataJSON: string(body),
		IsRead:       false,
		CreatedAt:    time.Now().UTC(),
	}
	return service.repository.CreateNotification(ctx, notification)
}

func notificationCopy(eventType string) (string, string) {
	switch eventType {
	case "payment.succeeded":
		return "Payment successful", "Your payment was completed successfully."
	case "payment.failed":
		return "Payment failed", "Your payment failed. Please try checkout again."
	case "order.created":
		return "Order created", "Your order has been created and is waiting for payment."
	case "order.payment_status_updated":
		return "Order updated", "Your order payment status was updated."
	case "inventory.released":
		return "Reservation released", "A reserved item was released back to stock."
	case "inventory.committed":
		return "Stock committed", "Your reserved stock was committed after payment."
	case "cart.checkout_prepared":
		return "Checkout ready", "Your reserved cart is ready for checkout."
	default:
		if strings.HasPrefix(eventType, "cart.") {
			return "Cart updated", "Your shopping cart was updated."
		}
		if strings.HasPrefix(eventType, "inventory.") {
			return "Inventory updated", fmt.Sprintf("Inventory event received: %s", eventType)
		}
		return "", ""
	}
}


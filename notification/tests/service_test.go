package tests

import (
	"context"
	"testing"

	"github.com/Tuananh165art/GoshopX/notification/internal"
	"github.com/Tuananh165art/GoshopX/notification/models"
	sharedevents "github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRepository struct {
	mock.Mock
}

type mockEmailSender struct{ recipient, subject, body string }

func (sender *mockEmailSender) Send(_ context.Context, recipient, subject, body string) error {
	sender.recipient = recipient
	sender.subject = subject
	sender.body = body
	return nil
}

type mockEmailResolver struct{}

func (mockEmailResolver) ResolveEmail(context.Context, uint64) (string, error) {
	return "customer@example.com", nil
}

func (m *MockRepository) Close() {}

func (m *MockRepository) CreateNotification(ctx context.Context, notification *models.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockRepository) ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error) {
	args := m.Called(ctx, accountID, skip, take)
	return args.Get(0).([]*models.Notification), args.Error(1)
}

func (m *MockRepository) CountUnread(ctx context.Context, accountID uint64) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) MarkRead(ctx context.Context, accountID uint64, notificationID uint64) (bool, error) {
	args := m.Called(ctx, accountID, notificationID)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockRepository) MarkAllRead(ctx context.Context, accountID uint64) (int64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(int64), args.Error(1)
}

func TestNotificationService_CreateFromEvent(t *testing.T) {
	repo := new(MockRepository)
	service := internal.NewNotificationService(repo)
	ctx := context.Background()

	repo.On("CreateNotification", ctx, mock.AnythingOfType("*models.Notification")).Return(nil).Once()

	err := service.CreateFromEvent(ctx, &sharedevents.Envelope{
		EventID:   "event-1",
		EventType: "payment.succeeded",
		AccountID: 1,
		Data:      map[string]any{"order_id": 10},
	})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestNotificationService_SendsEmailUsingAccountResolver(t *testing.T) {
	repo := new(MockRepository)
	sender := &mockEmailSender{}
	service := internal.NewNotificationServiceWithEmailResolver(repo, sender, mockEmailResolver{})
	ctx := context.Background()
	repo.On("CreateNotification", ctx, mock.AnythingOfType("*models.Notification")).Return(nil).Once()

	err := service.CreateFromEvent(ctx, &sharedevents.Envelope{EventID: "event-email", EventType: "payment.succeeded", AccountID: 7})

	assert.NoError(t, err)
	assert.Equal(t, "customer@example.com", sender.recipient)
	repo.AssertExpectations(t)
}

func TestNotificationService_EmailContainsOrderDetails(t *testing.T) {
	repo := new(MockRepository)
	sender := &mockEmailSender{}
	service := internal.NewNotificationServiceWithEmailResolver(repo, sender, mockEmailResolver{})
	ctx := context.Background()
	repo.On("CreateNotification", ctx, mock.AnythingOfType("*models.Notification")).Return(nil).Once()

	err := service.CreateFromEvent(ctx, &sharedevents.Envelope{
		EventID:   "event-order-details",
		EventType: "payment.succeeded",
		AccountID: 7,
		Data: map[string]any{
			"order_id":       float64(42),
			"total_price":    float64(300000),
			"currency":       "USD",
			"status":         "pending",
			"payment_status": "cod_pending",
			"products": []any{map[string]any{
				"id": "phone-1", "name": "Phone", "description": "Blue phone", "price": float64(300000), "quantity": float64(1),
			}},
		},
	})

	assert.NoError(t, err)
	assert.Contains(t, sender.subject, "#42")
	assert.Contains(t, sender.body, "Phone")
	assert.Contains(t, sender.body, "Blue phone")
	assert.Contains(t, sender.body, "300000 USD")
	assert.Contains(t, sender.body, "Số lượng: 1")
	assert.Contains(t, sender.body, "cod_pending")
	repo.AssertExpectations(t)
}

func TestNotificationService_DoesNotEmailPendingOrder(t *testing.T) {
	repo := new(MockRepository)
	sender := &mockEmailSender{}
	service := internal.NewNotificationServiceWithEmailResolver(repo, sender, mockEmailResolver{})
	ctx := context.Background()
	repo.On("CreateNotification", ctx, mock.AnythingOfType("*models.Notification")).Return(nil).Once()

	err := service.CreateFromEvent(ctx, &sharedevents.Envelope{
		EventID:   "event-pending",
		EventType: "order.created",
		AccountID: 7,
		Data:      map[string]any{"order_id": float64(42), "payment_status": "cod_pending"},
	})

	assert.NoError(t, err)
	assert.Empty(t, sender.recipient)
	repo.AssertExpectations(t)
}

func TestNotificationService_EmailsCODConfirmation(t *testing.T) {
	repo := new(MockRepository)
	sender := &mockEmailSender{}
	service := internal.NewNotificationServiceWithEmailResolver(repo, sender, mockEmailResolver{})
	ctx := context.Background()
	repo.On("CreateNotification", ctx, mock.AnythingOfType("*models.Notification")).Return(nil).Once()

	err := service.CreateFromEvent(ctx, &sharedevents.Envelope{
		EventID: "event-cod-created", EventType: "order.cod_created", AccountID: 7,
		Data: map[string]any{"order_id": float64(43), "payment_status": "cod_pending"},
	})

	assert.NoError(t, err)
	assert.Equal(t, "customer@example.com", sender.recipient)
	assert.Contains(t, sender.body, "cod_pending")
	repo.AssertExpectations(t)
}

func TestNotificationService_CountUnread(t *testing.T) {
	repo := new(MockRepository)
	service := internal.NewNotificationService(repo)
	ctx := context.Background()

	repo.On("CountUnread", ctx, uint64(1)).Return(int64(2), nil).Once()

	count, err := service.CountUnread(ctx, 1)

	assert.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

func TestNotificationService_MarkAllRead(t *testing.T) {
	repo := new(MockRepository)
	service := internal.NewNotificationService(repo)
	ctx := context.Background()

	repo.On("MarkAllRead", ctx, uint64(1)).Return(int64(3), nil).Once()

	count, err := service.MarkAllRead(ctx, 1)

	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)
}

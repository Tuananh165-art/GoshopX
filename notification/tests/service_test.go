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


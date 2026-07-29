package internal

import (
	"context"
	"time"

	"github.com/Tuananh165art/GoshopX/notification/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Close()
	CreateNotification(ctx context.Context, notification *models.Notification) error
	ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error)
	CountUnread(ctx context.Context, accountID uint64) (int64, error)
	MarkRead(ctx context.Context, accountID uint64, notificationID uint64) (bool, error)
	MarkAllRead(ctx context.Context, accountID uint64) (int64, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) (Repository, error) {
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	return &postgresRepository{db: db}, nil
}

func (repository *postgresRepository) Close() {
	sqlDB, err := repository.db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

func (repository *postgresRepository) CreateNotification(ctx context.Context, notification *models.Notification) error {
	return repository.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(notification).Error
}

func (repository *postgresRepository) ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error) {
	if take == 0 || take > 100 {
		take = 20
	}
	var notifications []*models.Notification
	err := repository.db.WithContext(ctx).
		Where("account_id = ?", accountID).
		Order("created_at DESC").
		Offset(int(skip)).
		Limit(int(take)).
		Find(&notifications).Error
	return notifications, err
}

func (repository *postgresRepository) CountUnread(ctx context.Context, accountID uint64) (int64, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&models.Notification{}).
		Where("account_id = ? AND is_read = ?", accountID, false).
		Count(&count).Error
	return count, err
}

func (repository *postgresRepository) MarkRead(ctx context.Context, accountID uint64, notificationID uint64) (bool, error) {
	now := time.Now().UTC()
	result := repository.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ? AND account_id = ? AND is_read = ?", notificationID, accountID, false).
		Updates(map[string]any{"is_read": true, "read_at": &now})
	return result.RowsAffected > 0, result.Error
}

func (repository *postgresRepository) MarkAllRead(ctx context.Context, accountID uint64) (int64, error) {
	now := time.Now().UTC()
	result := repository.db.WithContext(ctx).Model(&models.Notification{}).
		Where("account_id = ? AND is_read = ?", accountID, false).
		Updates(map[string]any{"is_read": true, "read_at": &now})
	return result.RowsAffected, result.Error
}


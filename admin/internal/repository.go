package internal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/models"
	"github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/migrations"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Close()
	ApplyEvent(ctx context.Context, topic string, event events.Envelope) error
	ListAudit(ctx context.Context, actorID uint64, action string, from, to time.Time, offset, limit int) ([]models.AuditEvent, error)
	GetDailyMetrics(ctx context.Context, from, to time.Time) ([]models.DailyMetric, error)
	GetProductMetrics(ctx context.Context, from, to time.Time, limit int) ([]models.ProductDailyMetric, error)
	Quarantine(ctx context.Context, topic string, partition int32, offset int64, payload []byte, reason string) error
	ReplayQuarantined(ctx context.Context, id uint) error
}

type postgresRepository struct{ db *gorm.DB }

func NewPostgresRepository(db *gorm.DB) (Repository, error) {
	if err := migrations.Run(db, "admin", "001_initial", func(db *gorm.DB) error {
		return db.AutoMigrate(&models.ProcessedEvent{}, &models.QuarantinedEvent{}, &models.AuditEvent{}, &models.DailyMetric{}, &models.ProductDailyMetric{})
	}); err != nil {
		return nil, err
	}
	if sqlDB, err := db.DB(); err != nil {
		return nil, err
	} else if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	return &postgresRepository{db: db}, nil
}

func (r *postgresRepository) Quarantine(ctx context.Context, topic string, partition int32, offset int64, payload []byte, reason string) error {
	return r.db.WithContext(ctx).Create(&models.QuarantinedEvent{Topic: topic, Partition: partition, Offset: offset, Payload: string(payload), Reason: reason, CreatedAt: time.Now().UTC()}).Error
}
func (r *postgresRepository) ReplayQuarantined(ctx context.Context, id uint) error {
	var record models.QuarantinedEvent
	if err := r.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return err
	}
	var event events.Envelope
	if err := json.Unmarshal([]byte(record.Payload), &event); err != nil {
		return err
	}
	if err := r.ApplyEvent(ctx, record.Topic, event); err != nil {
		return err
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&record).Update("replayed_at", &now).Error
}

func (r *postgresRepository) Close() {
	if db, err := r.db.DB(); err == nil {
		_ = db.Close()
	}
}

func (r *postgresRepository) ApplyEvent(ctx context.Context, topic string, event events.Envelope) error {
	if event.EventID == "" {
		return errors.New("event id is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		processed := models.ProcessedEvent{EventID: event.EventID, Topic: topic, OccurredAt: event.OccurredAt, CreatedAt: time.Now().UTC()}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&processed)
		if result.Error != nil {
			return result.Error
		}
		// A duplicate insert has zero affected rows; only the first event applies facts.
		if result.RowsAffected == 0 {
			return nil
		}
		return applyFacts(tx, event)
	})
}

func applyFacts(tx *gorm.DB, event events.Envelope) error {
	var data map[string]any
	body, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return err
	}
	day := event.OccurredAt.UTC().Truncate(24 * time.Hour)
	metric := models.DailyMetric{Day: day}
	if err := tx.FirstOrCreate(&metric, models.DailyMetric{Day: day}).Error; err != nil {
		return err
	}
	switch event.EventType {
	case "account_status_changed", "account_role_changed":
		audit := models.AuditEvent{EventID: event.EventID, ActorAccountID: event.AccountID, TargetAccountID: uint64(number(data["target_account_id"])), Action: event.EventType, Outcome: stringValue(data["outcome"]), RequestID: stringValue(data["request_id"]), OccurredAt: event.OccurredAt}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&audit).Error
	case "order.created":
		metric.OrderCount++
		metric.GMV += int64(number(data["total_price"]))
	case "payment.success", "payment.succeeded", "payment.settled":
		metric.PaymentAttempts++
		metric.SuccessfulPayments++
		metric.Revenue += int64(number(data["total_price"]))
	case "payment.failed", "payment.failure":
		metric.PaymentAttempts++
	default:
		return nil
	}
	return tx.Save(&metric).Error
}

func number(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func stringValue(v any) string { value, _ := v.(string); return value }

func (r *postgresRepository) ListAudit(ctx context.Context, actorID uint64, action string, from, to time.Time, offset, limit int) ([]models.AuditEvent, error) {
	query := r.db.WithContext(ctx).Order("occurred_at DESC").Offset(offset).Limit(limit)
	if actorID > 0 {
		query = query.Where("actor_account_id = ?", actorID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if !from.IsZero() {
		query = query.Where("occurred_at >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("occurred_at < ?", to)
	}
	var result []models.AuditEvent
	return result, query.Find(&result).Error
}

func (r *postgresRepository) GetDailyMetrics(ctx context.Context, from, to time.Time) ([]models.DailyMetric, error) {
	query := r.db.WithContext(ctx).Order("day ASC")
	if !from.IsZero() {
		query = query.Where("day >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("day < ?", to)
	}
	var result []models.DailyMetric
	return result, query.Find(&result).Error
}

func (r *postgresRepository) GetProductMetrics(ctx context.Context, from, to time.Time, limit int) ([]models.ProductDailyMetric, error) {
	query := r.db.WithContext(ctx).Order("gmv DESC").Limit(limit)
	if !from.IsZero() {
		query = query.Where("day >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("day < ?", to)
	}
	var result []models.ProductDailyMetric
	return result, query.Find(&result).Error
}

package internal

import (
	"context"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/models"
)

type Service interface {
	Dashboard(ctx context.Context, from, to time.Time, topProductsLimit int) (models.Dashboard, error)
	ListAudit(ctx context.Context, actorID uint64, action string, from, to time.Time, skip, take int) ([]models.AuditEvent, error)
}

type service struct{ repository Repository }

func NewService(repository Repository) Service { return &service{repository: repository} }

func (s *service) Dashboard(ctx context.Context, from, to time.Time, topProductsLimit int) (models.Dashboard, error) {
	daily, err := s.repository.GetDailyMetrics(ctx, from, to)
	if err != nil {
		return models.Dashboard{}, err
	}
	products, err := s.repository.GetProductMetrics(ctx, from, to, topProductsLimit)
	if err != nil {
		return models.Dashboard{}, err
	}
	result := models.Dashboard{From: from, To: to}
	for _, metric := range daily {
		result.GMV += metric.GMV
		result.Revenue += metric.Revenue
		result.OrderCount += metric.OrderCount
		result.PaymentAttempts += metric.PaymentAttempts
		result.SuccessfulPayments += metric.SuccessfulPayments
	}
	if result.OrderCount > 0 {
		result.AverageOrderValue = float64(result.GMV) / float64(result.OrderCount)
	}
	if result.PaymentAttempts > 0 {
		result.PaymentSuccessRate = float64(result.SuccessfulPayments) / float64(result.PaymentAttempts)
	}
	for _, metric := range products {
		result.TopProducts = append(result.TopProducts, models.ProductMetric{ProductID: metric.ProductID, Quantity: metric.Quantity, GMV: metric.GMV})
	}
	return result, nil
}

func (s *service) ListAudit(ctx context.Context, actorID uint64, action string, from, to time.Time, skip, take int) ([]models.AuditEvent, error) {
	if take <= 0 || take > 100 {
		take = 100
	}
	if skip < 0 {
		skip = 0
	}
	return s.repository.ListAudit(ctx, actorID, action, from, to, skip, take)
}

package internal

import (
	"sort"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/models"
)

// BuildDashboard calculates reporting KPIs from already deduplicated facts.
// Keeping this pure makes replay and reconciliation deterministic.
func BuildDashboard(from, to time.Time, orders []models.OrderFact, payments []models.PaymentFact, limit int) models.Dashboard {
	result := models.Dashboard{From: from, To: to}
	seenOrders := map[uint64]struct{}{}
	products := map[string]*models.ProductMetric{}

	for _, order := range orders {
		if order.OccurredAt.Before(from) || !order.OccurredAt.Before(to) || order.Status != "paid" {
			continue
		}
		result.GMV += order.GMV
		if _, exists := seenOrders[order.OrderID]; !exists {
			seenOrders[order.OrderID] = struct{}{}
			result.OrderCount++
		}
		metric := products[order.ProductID]
		if metric == nil {
			metric = &models.ProductMetric{ProductID: order.ProductID}
			products[order.ProductID] = metric
		}
		metric.Quantity += order.Quantity
		metric.GMV += order.GMV
	}

	for _, payment := range payments {
		if payment.OccurredAt.Before(from) || !payment.OccurredAt.Before(to) {
			continue
		}
		result.PaymentAttempts++
		if payment.Status == "success" || payment.Status == "settled" {
			result.SuccessfulPayments++
			result.Revenue += payment.Amount
		}
	}

	if result.OrderCount > 0 {
		result.AverageOrderValue = float64(result.GMV) / float64(result.OrderCount)
	}
	if result.PaymentAttempts > 0 {
		result.PaymentSuccessRate = float64(result.SuccessfulPayments) / float64(result.PaymentAttempts)
	}
	for _, metric := range products {
		result.TopProducts = append(result.TopProducts, *metric)
	}
	sort.Slice(result.TopProducts, func(i, j int) bool {
		if result.TopProducts[i].GMV == result.TopProducts[j].GMV {
			return result.TopProducts[i].ProductID < result.TopProducts[j].ProductID
		}
		return result.TopProducts[i].GMV > result.TopProducts[j].GMV
	})
	if limit > 0 && len(result.TopProducts) > limit {
		result.TopProducts = result.TopProducts[:limit]
	}
	return result
}

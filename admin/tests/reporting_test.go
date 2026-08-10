package tests

import (
	"testing"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/internal"
	"github.com/Tuananh165art/GoshopX/admin/models"
	"github.com/stretchr/testify/assert"
)

func TestBuildDashboard(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	orders := []models.OrderFact{
		{EventID: "o-1", OccurredAt: from.Add(24 * time.Hour), OrderID: 10, ProductID: "p-1", Quantity: 2, GMV: 3000, Status: "paid"},
		{EventID: "o-1-line-2", OccurredAt: from.Add(24 * time.Hour), OrderID: 10, ProductID: "p-2", Quantity: 1, GMV: 1000, Status: "paid"},
		{EventID: "o-2", OccurredAt: from.Add(48 * time.Hour), OrderID: 11, ProductID: "p-1", Quantity: 1, GMV: 2000, Status: "pending"},
	}
	payments := []models.PaymentFact{
		{EventID: "pay-1", OccurredAt: from.Add(24 * time.Hour), PaymentID: "pay-1", OrderID: 10, Amount: 4000, Status: "success"},
		{EventID: "pay-2", OccurredAt: from.Add(48 * time.Hour), PaymentID: "pay-2", OrderID: 11, Amount: 2000, Status: "failed"},
	}

	result := internal.BuildDashboard(from, to, orders, payments, 1)
	assert.Equal(t, int64(4000), result.GMV)
	assert.Equal(t, uint64(1), result.OrderCount)
	assert.Equal(t, float64(4000), result.AverageOrderValue)
	assert.Equal(t, int64(4000), result.Revenue)
	assert.Equal(t, uint64(2), result.PaymentAttempts)
	assert.Equal(t, float64(0.5), result.PaymentSuccessRate)
	assert.Len(t, result.TopProducts, 1)
	assert.Equal(t, "p-1", result.TopProducts[0].ProductID)
}

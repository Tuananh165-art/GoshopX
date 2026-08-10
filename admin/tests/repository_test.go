package tests

import (
	"context"
	"testing"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/internal"
	"github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRepositoryApplyEventIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	repo, err := internal.NewPostgresRepository(db)
	require.NoError(t, err)
	defer repo.Close()
	event := events.Envelope{Version: "v1", EventID: "evt-1", EventType: "order.created", OccurredAt: time.Now().UTC(), Data: map[string]any{"total_price": 1200}}
	require.NoError(t, repo.ApplyEvent(context.Background(), "order_events", event))
	require.NoError(t, repo.ApplyEvent(context.Background(), "order_events", event))
	metrics, err := repo.GetDailyMetrics(context.Background(), time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	if metrics[0].GMV != 1200 || metrics[0].OrderCount != 1 {
		t.Fatalf("unexpected metric: %+v", metrics[0])
	}
}

func TestRepositoryProjectsAdminAuditEvent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	repo, err := internal.NewPostgresRepository(db)
	require.NoError(t, err)
	defer repo.Close()

	event := events.Envelope{Version: "v1", EventID: "audit-1", EventType: "account_status_changed", AccountID: 1, OccurredAt: time.Now().UTC(), Data: map[string]any{"target_account_id": 2, "outcome": "success", "request_id": "request-1"}}
	require.NoError(t, repo.ApplyEvent(context.Background(), "admin_events", event))
	items, err := repo.ListAudit(context.Background(), 1, "account_status_changed", time.Time{}, time.Time{}, 0, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, uint64(2), items[0].TargetAccountID)
}

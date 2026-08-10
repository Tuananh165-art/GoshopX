package client

import (
	"context"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/models"
	"github.com/Tuananh165art/GoshopX/admin/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.AdminServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, service: pb.NewAdminServiceClient(conn)}, nil
}
func (c *Client) Close() { _ = c.conn.Close() }

func (c *Client) Dashboard(ctx context.Context, from, to time.Time, limit int) (*models.Dashboard, error) {
	result, err := c.service.GetDashboard(ctx, &pb.DashboardRequest{FromUnix: from.Unix(), ToUnix: to.Unix(), TopProductsLimit: uint64(limit)})
	if err != nil {
		return nil, err
	}
	dashboard := &models.Dashboard{From: from, To: to, GMV: result.Gmv, Revenue: result.Revenue, OrderCount: result.OrderCount, AverageOrderValue: result.AverageOrderValue, PaymentAttempts: result.PaymentAttempts, SuccessfulPayments: result.SuccessfulPayments, PaymentSuccessRate: result.PaymentSuccessRate}
	for _, metric := range result.TopProducts {
		dashboard.TopProducts = append(dashboard.TopProducts, models.ProductMetric{ProductID: metric.ProductId, Quantity: metric.Quantity, GMV: metric.Gmv})
	}
	return dashboard, nil
}

func (c *Client) ListAudit(ctx context.Context, actorID uint64, action string, from, to time.Time, skip, take int) ([]models.AuditEvent, error) {
	result, err := c.service.ListAuditEvents(ctx, &pb.ListAuditEventsRequest{ActorAccountId: actorID, Action: action, FromUnix: from.Unix(), ToUnix: to.Unix(), Skip: uint64(skip), Take: uint64(take)})
	if err != nil {
		return nil, err
	}
	items := make([]models.AuditEvent, 0, len(result.Events))
	for _, item := range result.Events {
		items = append(items, models.AuditEvent{EventID: item.EventId, ActorAccountID: item.ActorAccountId, TargetAccountID: item.TargetAccountId, Action: item.Action, Outcome: item.Outcome, RequestID: item.RequestId, OccurredAt: time.Unix(item.OccurredAtUnix, 0).UTC()})
	}
	return items, nil
}

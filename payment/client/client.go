package client

import (
	"context"
	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"log"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.PaymentServiceClient
}

func NewClient(url string) (*Client, error) {
	c, e := grpc.NewClient(url, append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, observability.GRPCClientOptions()...)...)
	if e != nil {
		return nil, e
	}
	return &Client{c, pb.NewPaymentServiceClient(c)}, nil
}
func (c *Client) Close() {
	if e := c.conn.Close(); e != nil {
		log.Println(e)
	}
}
func (c *Client) CreateCustomerPortalSession(ctx context.Context, user uint64, email, name string) (string, error) {
	x, e := c.service.CreateCustomerPortalSession(ctx, &pb.CustomerPortalRequest{UserId: user, Email: &email, Name: &name})
	if e != nil {
		return "", e
	}
	return x.Value, nil
}
func (c *Client) CreateCheckoutSession(ctx context.Context, order, user int, email, name, redirect, clientIP string, items []*pb.CheckoutCartItem, res []string) (string, error) {
	x, e := c.service.CreateCheckoutSession(ctx, &pb.CheckoutRequest{UserId: uint64(user), Email: email, Name: name, RedirectURL: redirect, ClientIp: clientIP, Products: items, OrderId: uint64(order), ReservationIds: res})
	if e != nil {
		return "", e
	}
	return x.Value, nil
}
func (c *Client) ListTransactions(ctx context.Context, status string, order, skip, take uint64) ([]*models.Transaction, error) {
	x, e := c.service.ListTransactions(ctx, &pb.ListTransactionsRequest{Status: status, OrderId: order, Skip: skip, Take: take})
	if e != nil {
		return nil, e
	}
	out := make([]*models.Transaction, 0, len(x.Transactions))
	for _, i := range x.Transactions {
		out = append(out, &models.Transaction{OrderId: i.OrderId, UserId: i.UserId, PaymentId: i.PaymentId, TotalPrice: i.TotalPrice, SettledPrice: i.SettledPrice, Currency: i.Currency, Status: i.Status})
	}
	return out, nil
}
func (c *Client) RequestRefund(ctx context.Context, id, reason, key string) (*pb.RefundResponse, error) {
	return c.service.RequestRefund(ctx, &pb.RefundRequest{PaymentId: id, Reason: reason, IdempotencyKey: key})
}
func (c *Client) ReconcileTransaction(ctx context.Context, id string) (*models.Transaction, error) {
	x, e := c.service.ReconcileTransaction(ctx, wrapperspb.String(id))
	if e != nil {
		return nil, e
	}
	return &models.Transaction{OrderId: x.OrderId, UserId: x.UserId, PaymentId: x.PaymentId, TotalPrice: x.TotalPrice, SettledPrice: x.SettledPrice, Currency: x.Currency, Status: x.Status}, nil
}

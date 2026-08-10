package internal

import (
	"context"
	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type grpcServer struct {
	pb.UnimplementedPaymentServiceServer
	service Service
}

func (s *grpcServer) CreateCheckoutSession(ctx context.Context, r *pb.CheckoutRequest) (*wrapperspb.StringValue, error) {
	u, e := s.service.CreateCheckoutSession(ctx, r.UserId, r.RedirectURL, r.ClientIp, r.Products, r.OrderId, r.ReservationIds)
	if e != nil {
		return nil, e
	}
	return wrapperspb.String(u), nil
}
func (s *grpcServer) CreateCustomerPortalSession(ctx context.Context, r *pb.CustomerPortalRequest) (*wrapperspb.StringValue, error) {
	u, e := s.service.CreateCustomerPortalSession(ctx, &models.Customer{UserId: r.UserId})
	if e != nil {
		return nil, e
	}
	return wrapperspb.String(u), nil
}
func encodeTransaction(x *models.Transaction) *pb.Transaction {
	return &pb.Transaction{Id: uint64(x.ID), OrderId: x.OrderId, UserId: x.UserId, PaymentId: x.PaymentId, TotalPrice: x.TotalPrice, SettledPrice: x.SettledPrice, Currency: x.Currency, Status: x.Status}
}
func (s *grpcServer) ListTransactions(ctx context.Context, r *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	xs, e := s.service.ListTransactions(ctx, r.Status, r.OrderId, r.Skip, r.Take)
	if e != nil {
		return nil, e
	}
	out := &pb.ListTransactionsResponse{}
	for _, x := range xs {
		out.Transactions = append(out.Transactions, encodeTransaction(x))
	}
	return out, nil
}
func (s *grpcServer) RequestRefund(ctx context.Context, r *pb.RefundRequest) (*pb.RefundResponse, error) {
	x, e := s.service.RequestRefund(ctx, r.PaymentId, r.Reason, r.IdempotencyKey)
	if e != nil {
		return nil, e
	}
	return &pb.RefundResponse{ProviderRefundId: x.ProviderRefundID, Status: x.Status, Amount: x.Amount}, nil
}
func (s *grpcServer) ReconcileTransaction(ctx context.Context, r *wrapperspb.StringValue) (*pb.Transaction, error) {
	x, e := s.service.ReconcileTransaction(ctx, r.Value)
	if e != nil {
		return nil, e
	}
	return encodeTransaction(x), nil
}

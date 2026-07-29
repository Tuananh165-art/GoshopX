package internal

import (
	"context"

	order "github.com/Tuananh165art/GoshopX/order/client"
	"github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type grpcServer struct {
	pb.UnimplementedPaymentServiceServer
	service     Service
	orderClient *order.Client
}

func (s *grpcServer) CreateCheckoutSession(ctx context.Context, request *pb.CheckoutRequest) (*wrapperspb.StringValue, error) {
	customer, err := s.service.FindOrCreateCustomer(ctx, request.UserId, request.Email, request.Name)
	if err != nil {
		return nil, err
	}

	checkoutUrl, err := s.service.CreateCheckoutSession(ctx, request.UserId, customer.CustomerId, request.RedirectURL, request.Products, request.OrderId, request.ReservationIds)
	if err != nil {
		return nil, err
	}

	return &wrapperspb.StringValue{
		Value: checkoutUrl,
	}, nil
}

func (s *grpcServer) CreateCustomerPortalSession(ctx context.Context, request *pb.CustomerPortalRequest) (*wrapperspb.StringValue, error) {
	customer, err := s.service.FindOrCreateCustomer(ctx, request.UserId, *request.Email, *request.Name)

	if err != nil {
		return nil, err
	}

	link, err := s.service.CreateCustomerPortalSession(ctx, customer)

	if err != nil {
		return nil, err
	}
	return &wrapperspb.StringValue{
		Value: link,
	}, nil
}


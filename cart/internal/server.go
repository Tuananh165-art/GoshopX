package internal

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/Tuananh165art/GoshopX/cart/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type grpcServer struct {
	pb.UnimplementedCartServiceServer
	service Service
}

func ListenGRPC(service Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	server := grpc.NewServer(observability.GRPCServerOptions("cart")...)
	pb.RegisterCartServiceServer(server, &grpcServer{
		UnimplementedCartServiceServer: pb.UnimplementedCartServiceServer{},
		service:                        service,
	})
	reflection.Register(server)
	return server.Serve(lis)
}

func (server *grpcServer) GetCart(ctx context.Context, request *pb.GetCartRequest) (*pb.Cart, error) {
	cart, err := server.service.GetCart(ctx, request.AccountId)
	if err != nil {
		return nil, err
	}
	return encodeCart(cart), nil
}

func (server *grpcServer) AddCartItem(ctx context.Context, request *pb.AddCartItemRequest) (*pb.Cart, error) {
	cart, err := server.service.AddCartItem(ctx, request.AccountId, request.ProductId, request.Quantity)
	if err != nil {
		return nil, err
	}
	return encodeCart(cart), nil
}

func (server *grpcServer) UpsertCartItem(ctx context.Context, request *pb.UpsertCartItemRequest) (*pb.Cart, error) {
	cart, err := server.service.UpsertCartItem(ctx, request.AccountId, request.ProductId, request.Quantity)
	if err != nil {
		return nil, err
	}
	return encodeCart(cart), nil
}

func (server *grpcServer) RemoveCartItem(ctx context.Context, request *pb.RemoveCartItemRequest) (*pb.Cart, error) {
	cart, err := server.service.RemoveCartItem(ctx, request.AccountId, request.ProductId)
	if err != nil {
		return nil, err
	}
	return encodeCart(cart), nil
}

func (server *grpcServer) ClearCart(ctx context.Context, request *pb.ClearCartRequest) (*pb.BooleanResponse, error) {
	if err := server.service.ClearCart(ctx, request.AccountId); err != nil {
		return nil, err
	}
	return &pb.BooleanResponse{Ok: true}, nil
}

func (server *grpcServer) PrepareCheckout(ctx context.Context, request *pb.PrepareCheckoutRequest) (*pb.Cart, error) {
	cart, err := server.service.PrepareCheckoutAtVersion(ctx, request.AccountId, request.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return encodeCart(cart), nil
}

func (server *grpcServer) CompleteCheckout(ctx context.Context, request *pb.ClearCartRequest) (*pb.BooleanResponse, error) {
	if err := server.service.CompleteCheckout(ctx, request.AccountId); err != nil {
		return nil, err
	}
	return &pb.BooleanResponse{Ok: true}, nil
}

func (server *grpcServer) CreateCartProposal(ctx context.Context, request *pb.CreateCartProposalRequest) (*pb.CartProposal, error) {
	items := make([]models.ProposalItem, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, models.ProposalItem{ProductID: item.ProductId, Quantity: item.Quantity})
	}
	proposal, err := server.service.CreateCartProposal(ctx, request.AccountId, request.ExpectedVersion, request.IdempotencyKey, items)
	if err != nil {
		return nil, err
	}
	return encodeCartProposal(proposal), nil
}

func (server *grpcServer) ConfirmCartProposal(ctx context.Context, request *pb.ConfirmCartProposalRequest) (*pb.Cart, error) {
	cart, err := server.service.ConfirmCartProposal(ctx, request.AccountId, request.ProposalId)
	if err != nil {
		return nil, err
	}
	return encodeCart(cart), nil
}

func encodeCartProposal(proposal *models.CartProposal) *pb.CartProposal {
	result := &pb.CartProposal{Id: proposal.ID, AccountId: proposal.AccountID, BaseVersion: proposal.BaseVersion, ExpiresAtUnix: proposal.ExpiresAt.Unix(), Items: make([]*pb.CartProposalItem, 0, len(proposal.Items))}
	for _, item := range proposal.Items {
		result.Items = append(result.Items, &pb.CartProposalItem{ProductId: item.ProductID, Quantity: item.Quantity})
	}
	if proposal.Result != nil {
		result.Result = encodeCart(proposal.Result)
	}
	return result
}

func encodeCart(cart *models.Cart) *pb.Cart {
	result := &pb.Cart{
		AccountId:     cart.AccountID,
		Version:       cart.Version,
		Items:         make([]*pb.CartItem, 0, len(cart.Items)),
		ExpiresAtUnix: cart.ExpiresAt.Unix(),
	}
	for _, item := range cart.Items {
		result.Items = append(result.Items, &pb.CartItem{
			ProductId:         item.ProductID,
			Quantity:          item.Quantity,
			ReservationId:     item.ReservationID,
			ReservedUntilUnix: item.ReservedUntil.Unix(),
		})
	}
	return result
}

func decodeCart(cart *pb.Cart) *models.Cart {
	result := &models.Cart{
		AccountID: cart.AccountId,
		Version:   cart.Version,
		Items:     make([]models.CartItem, 0, len(cart.Items)),
		ExpiresAt: time.Unix(cart.ExpiresAtUnix, 0).UTC(),
	}
	for _, item := range cart.Items {
		result.Items = append(result.Items, models.CartItem{
			ProductID:     item.ProductId,
			Quantity:      item.Quantity,
			ReservationID: item.ReservationId,
			ReservedUntil: time.Unix(item.ReservedUntilUnix, 0).UTC(),
		})
	}
	return result
}

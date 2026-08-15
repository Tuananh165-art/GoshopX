package internal

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Tuananh165art/GoshopX/inventory/models"
	"github.com/Tuananh165art/GoshopX/inventory/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type grpcServer struct {
	pb.UnimplementedInventoryServiceServer
	service Service
}

func ListenGRPC(service Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	server := grpc.NewServer(observability.GRPCServerOptions("inventory")...)
	pb.RegisterInventoryServiceServer(server, &grpcServer{
		UnimplementedInventoryServiceServer: pb.UnimplementedInventoryServiceServer{},
		service:                             service,
	})
	reflection.Register(server)
	return server.Serve(lis)
}

func (server *grpcServer) UpsertStock(ctx context.Context, request *pb.UpsertStockRequest) (*pb.UpsertStockResponse, error) {
	stock, availability, err := server.service.UpsertStock(ctx, request.ProductId, request.Quantity, request.ReorderLevel)
	if err != nil {
		return nil, err
	}
	return &pb.UpsertStockResponse{
		Stock:        encodeStock(stock),
		Availability: encodeAvailability(availability),
	}, nil
}

func (server *grpcServer) GetAvailability(ctx context.Context, request *pb.AvailabilityRequest) (*pb.Availability, error) {
	availability, err := server.service.GetAvailability(ctx, request.ProductId)
	if err != nil {
		return nil, err
	}
	return encodeAvailability(availability), nil
}

func (server *grpcServer) ReserveStock(ctx context.Context, request *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	reservation, availability, err := server.service.ReserveStock(ctx, request.ReservationId, request.AccountId, request.ProductId, request.Quantity, time.Duration(request.TtlSeconds)*time.Second, request.Source)
	if err != nil {
		return nil, err
	}
	return &pb.ReserveStockResponse{
		Reservation:  encodeReservation(reservation),
		Availability: encodeAvailability(availability),
	}, nil
}

func (server *grpcServer) ReleaseReservation(ctx context.Context, request *pb.ReservationRequest) (*pb.ReservationResponse, error) {
	reservation, availability, err := server.service.ReleaseReservation(ctx, request.ReservationId)
	if err != nil {
		return nil, err
	}
	return &pb.ReservationResponse{
		Reservation:  encodeReservation(reservation),
		Availability: encodeAvailability(availability),
	}, nil
}

func (server *grpcServer) CommitReservation(ctx context.Context, request *pb.ReservationRequest) (*pb.ReservationResponse, error) {
	reservation, availability, err := server.service.CommitReservation(ctx, request.ReservationId)
	if err != nil {
		return nil, err
	}
	return &pb.ReservationResponse{
		Reservation:  encodeReservation(reservation),
		Availability: encodeAvailability(availability),
	}, nil
}

func (server *grpcServer) ListLowStock(ctx context.Context, request *pb.ListLowStockRequest) (*pb.ListLowStockResponse, error) {
	stocks, err := server.service.ListLowStock(ctx, request.Limit)
	if err != nil {
		return nil, err
	}
	response := &pb.ListLowStockResponse{Stocks: make([]*pb.Stock, 0, len(stocks))}
	for _, stock := range stocks {
		response.Stocks = append(response.Stocks, encodeStock(stock))
	}
	return response, nil
}

func (server *grpcServer) AdjustStock(ctx context.Context, request *pb.AdjustStockRequest) (*pb.UpsertStockResponse, error) {
	stock, availability, err := server.service.AdjustStock(ctx, request.ProductId, request.Delta, request.ReorderLevel, request.Reason)
	if err != nil {
		return nil, err
	}
	return &pb.UpsertStockResponse{Stock: encodeStock(stock), Availability: encodeAvailability(availability)}, nil
}
func (server *grpcServer) ListReservations(ctx context.Context, request *pb.ListReservationsRequest) (*pb.ListReservationsResponse, error) {
	items, err := server.service.ListReservations(ctx, request.Status, request.Skip, request.Take)
	if err != nil {
		return nil, err
	}
	response := &pb.ListReservationsResponse{Reservations: make([]*pb.Reservation, 0, len(items))}
	for _, item := range items {
		response.Reservations = append(response.Reservations, encodeReservation(item))
	}
	return response, nil
}

func encodeStock(stock *models.Stock) *pb.Stock {
	return &pb.Stock{
		ProductId:    stock.ProductID,
		Quantity:     stock.Quantity,
		ReorderLevel: stock.ReorderLevel,
	}
}

func encodeAvailability(availability *models.Availability) *pb.Availability {
	return &pb.Availability{
		ProductId:         availability.ProductID,
		TotalQuantity:     availability.TotalQuantity,
		ReservedQuantity:  availability.ReservedQuantity,
		AvailableQuantity: availability.AvailableQuantity,
		ReorderLevel:      availability.ReorderLevel,
		LowStock:          availability.HasActiveLowStock,
	}
}

func encodeReservation(reservation *models.StockReservation) *pb.Reservation {
	return &pb.Reservation{
		ReservationId: reservation.ReservationID,
		AccountId:     reservation.AccountID,
		ProductId:     reservation.ProductID,
		Quantity:      reservation.Quantity,
		Status:        string(reservation.Status),
		Source:        reservation.Source,
		ExpiresAtUnix: reservation.ExpiresAt.Unix(),
	}
}

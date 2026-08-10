package client

import (
	"context"
	"log"
	"time"

	"github.com/Tuananh165art/GoshopX/inventory/models"
	"github.com/Tuananh165art/GoshopX/inventory/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.InventoryServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, service: pb.NewInventoryServiceClient(conn)}, nil
}

func (client *Client) Close() {
	if err := client.conn.Close(); err != nil {
		log.Println(err)
	}
}

func (client *Client) UpsertStock(ctx context.Context, productID string, quantity, reorderLevel int32) (*models.Stock, *models.Availability, error) {
	response, err := client.service.UpsertStock(ctx, &pb.UpsertStockRequest{
		ProductId:    productID,
		Quantity:     quantity,
		ReorderLevel: reorderLevel,
	})
	if err != nil {
		return nil, nil, err
	}
	return decodeStock(response.Stock), decodeAvailability(response.Availability), nil
}

func (client *Client) GetAvailability(ctx context.Context, productID string) (*models.Availability, error) {
	response, err := client.service.GetAvailability(ctx, &pb.AvailabilityRequest{ProductId: productID})
	if err != nil {
		return nil, err
	}
	return decodeAvailability(response), nil
}

func (client *Client) ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, ttl time.Duration, source string) (*models.StockReservation, *models.Availability, error) {
	response, err := client.service.ReserveStock(ctx, &pb.ReserveStockRequest{
		ReservationId: reservationID,
		AccountId:     accountID,
		ProductId:     productID,
		Quantity:      quantity,
		TtlSeconds:    int64(ttl / time.Second),
		Source:        source,
	})
	if err != nil {
		return nil, nil, err
	}
	return decodeReservation(response.Reservation), decodeAvailability(response.Availability), nil
}

func (client *Client) ReleaseReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	response, err := client.service.ReleaseReservation(ctx, &pb.ReservationRequest{ReservationId: reservationID})
	if err != nil {
		return nil, nil, err
	}
	return decodeReservation(response.Reservation), decodeAvailability(response.Availability), nil
}

func (client *Client) CommitReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	response, err := client.service.CommitReservation(ctx, &pb.ReservationRequest{ReservationId: reservationID})
	if err != nil {
		return nil, nil, err
	}
	return decodeReservation(response.Reservation), decodeAvailability(response.Availability), nil
}

func (client *Client) ListLowStock(ctx context.Context, limit uint64) ([]*models.Stock, error) {
	response, err := client.service.ListLowStock(ctx, &pb.ListLowStockRequest{Limit: limit})
	if err != nil {
		return nil, err
	}
	stocks := make([]*models.Stock, 0, len(response.Stocks))
	for _, stock := range response.Stocks {
		stocks = append(stocks, decodeStock(stock))
	}
	return stocks, nil
}

func (client *Client) AdjustStock(ctx context.Context, productID string, delta, reorderLevel int32, reason string) (*models.Stock, *models.Availability, error) {
	response, err := client.service.AdjustStock(ctx, &pb.AdjustStockRequest{ProductId: productID, Delta: delta, ReorderLevel: reorderLevel, Reason: reason})
	if err != nil {
		return nil, nil, err
	}
	return decodeStock(response.Stock), decodeAvailability(response.Availability), nil
}
func (client *Client) ListReservations(ctx context.Context, status string, skip, take uint64) ([]*models.StockReservation, error) {
	response, err := client.service.ListReservations(ctx, &pb.ListReservationsRequest{Status: status, Skip: skip, Take: take})
	if err != nil {
		return nil, err
	}
	items := make([]*models.StockReservation, 0, len(response.Reservations))
	for _, item := range response.Reservations {
		items = append(items, decodeReservation(item))
	}
	return items, nil
}

func decodeStock(stock *pb.Stock) *models.Stock {
	if stock == nil {
		return nil
	}
	return &models.Stock{
		ProductID:    stock.ProductId,
		Quantity:     stock.Quantity,
		ReorderLevel: stock.ReorderLevel,
	}
}

func decodeAvailability(availability *pb.Availability) *models.Availability {
	if availability == nil {
		return nil
	}
	return &models.Availability{
		ProductID:         availability.ProductId,
		TotalQuantity:     availability.TotalQuantity,
		ReservedQuantity:  availability.ReservedQuantity,
		AvailableQuantity: availability.AvailableQuantity,
		ReorderLevel:      availability.ReorderLevel,
		HasActiveLowStock: availability.LowStock,
	}
}

func decodeReservation(reservation *pb.Reservation) *models.StockReservation {
	if reservation == nil {
		return nil
	}
	return &models.StockReservation{
		ReservationID: reservation.ReservationId,
		AccountID:     reservation.AccountId,
		ProductID:     reservation.ProductId,
		Quantity:      reservation.Quantity,
		Status:        models.ReservationStatus(reservation.Status),
		Source:        reservation.Source,
		ExpiresAt:     time.Unix(reservation.ExpiresAtUnix, 0).UTC(),
	}
}

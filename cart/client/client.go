package client

import (
	"context"
	"log"
	"time"

	"github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/Tuananh165art/GoshopX/cart/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.CartServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, service: pb.NewCartServiceClient(conn)}, nil
}

func (client *Client) Close() {
	if err := client.conn.Close(); err != nil {
		log.Println(err)
	}
}

func (client *Client) GetCart(ctx context.Context, accountID uint64) (*models.Cart, error) {
	response, err := client.service.GetCart(ctx, &pb.GetCartRequest{AccountId: accountID})
	if err != nil {
		return nil, err
	}
	return decodeCart(response), nil
}

func (client *Client) AddCartItem(ctx context.Context, accountID uint64, productID string, quantity int32) (*models.Cart, error) {
	response, err := client.service.AddCartItem(ctx, &pb.AddCartItemRequest{AccountId: accountID, ProductId: productID, Quantity: quantity})
	if err != nil {
		return nil, err
	}
	return decodeCart(response), nil
}

func (client *Client) UpsertCartItem(ctx context.Context, accountID uint64, productID string, quantity int32) (*models.Cart, error) {
	response, err := client.service.UpsertCartItem(ctx, &pb.UpsertCartItemRequest{
		AccountId: accountID,
		ProductId: productID,
		Quantity:  quantity,
	})
	if err != nil {
		return nil, err
	}
	return decodeCart(response), nil
}

func (client *Client) RemoveCartItem(ctx context.Context, accountID uint64, productID string) (*models.Cart, error) {
	response, err := client.service.RemoveCartItem(ctx, &pb.RemoveCartItemRequest{
		AccountId: accountID,
		ProductId: productID,
	})
	if err != nil {
		return nil, err
	}
	return decodeCart(response), nil
}

func (client *Client) ClearCart(ctx context.Context, accountID uint64) error {
	_, err := client.service.ClearCart(ctx, &pb.ClearCartRequest{AccountId: accountID})
	return err
}

func (client *Client) CompleteCheckout(ctx context.Context, accountID uint64) error {
	_, err := client.service.CompleteCheckout(ctx, &pb.ClearCartRequest{AccountId: accountID})
	return err
}

func (client *Client) PrepareCheckout(ctx context.Context, accountID uint64) (*models.Cart, error) {
	response, err := client.service.PrepareCheckout(ctx, &pb.PrepareCheckoutRequest{AccountId: accountID})
	if err != nil {
		return nil, err
	}
	return decodeCart(response), nil
}

func decodeCart(cart *pb.Cart) *models.Cart {
	result := &models.Cart{
		AccountID: cart.AccountId,
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

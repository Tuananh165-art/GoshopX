package client

import (
	"context"
	"log"
	"time"

	"github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/Tuananh165art/GoshopX/cart/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.CartServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, observability.GRPCClientOptions()...)...)
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
	return client.PrepareCheckoutAtVersion(ctx, accountID, 0)
}

func (client *Client) PrepareCheckoutAtVersion(ctx context.Context, accountID, expectedVersion uint64) (*models.Cart, error) {
	response, err := client.service.PrepareCheckout(ctx, &pb.PrepareCheckoutRequest{AccountId: accountID, ExpectedVersion: expectedVersion})
	if err != nil {
		return nil, err
	}
	return decodeCart(response), nil
}

func (client *Client) CreateCartProposal(ctx context.Context, accountID, expectedVersion uint64, idempotencyKey string, items []models.ProposalItem) (*models.CartProposal, error) {
	request := &pb.CreateCartProposalRequest{AccountId: accountID, ExpectedVersion: expectedVersion, IdempotencyKey: idempotencyKey, Items: make([]*pb.CartProposalItem, 0, len(items))}
	for _, item := range items {
		request.Items = append(request.Items, &pb.CartProposalItem{ProductId: item.ProductID, Quantity: item.Quantity})
	}
	proposal, err := client.service.CreateCartProposal(ctx, request)
	if err != nil {
		return nil, err
	}
	return decodeCartProposal(proposal), nil
}

func (client *Client) ConfirmCartProposal(ctx context.Context, accountID uint64, proposalID string) (*models.Cart, error) {
	cart, err := client.service.ConfirmCartProposal(ctx, &pb.ConfirmCartProposalRequest{AccountId: accountID, ProposalId: proposalID})
	if err != nil {
		return nil, err
	}
	return decodeCart(cart), nil
}

func decodeCartProposal(proposal *pb.CartProposal) *models.CartProposal {
	result := &models.CartProposal{ID: proposal.Id, AccountID: proposal.AccountId, BaseVersion: proposal.BaseVersion, ExpiresAt: time.Unix(proposal.ExpiresAtUnix, 0).UTC(), Items: make([]models.ProposalItem, 0, len(proposal.Items))}
	for _, item := range proposal.Items {
		result.Items = append(result.Items, models.ProposalItem{ProductID: item.ProductId, Quantity: item.Quantity})
	}
	if proposal.Result != nil {
		result.Result = decodeCart(proposal.Result)
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

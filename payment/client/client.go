package client

import (
	"context"
	"log"

	"github.com/rasadov/EcommerceAPI/payment/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.PaymentServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	C := pb.NewPaymentServiceClient(conn)
	return &Client{conn, C}, nil
}

func (client *Client) Close() {
	err := client.conn.Close()
	if err != nil {
		log.Println(err)
	}
}

func (client *Client) CreateCustomerPortalSession(ctx context.Context, userId uint64, email, name string) (string, error) {
	res, err := client.service.CreateCustomerPortalSession(ctx, &pb.CustomerPortalRequest{
		UserId: userId,
		Email:  &email,
		Name:   &name,
	})
	if err != nil {
		log.Println(err)
		return "", err
	}
	return res.Value, nil
}

func (client *Client) CreateCheckoutSession(ctx context.Context, orderId, userId int,
	email, name, redirectUrl string, products []*pb.CartItem) (string, error) {
	res, err := client.service.CreateCheckoutSession(ctx, &pb.CheckoutRequest{
		UserId:      uint64(userId),
		Email:       email,
		Name:        name,
		RedirectURL: redirectUrl,
		Products:    products,
		OrderId:     uint64(orderId),
	})
	if err != nil {
		log.Println(err)
		return "", err
	}
	return res.Value, nil
}

package client

import (
	"context"
	"log"

	"github.com/Tuananh165art/GoshopX/account/models"
	"github.com/Tuananh165art/GoshopX/account/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.AccountServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	C := pb.NewAccountServiceClient(conn)
	return &Client{conn, C}, nil
}

func (client *Client) Close() {
	err := client.conn.Close()
	if err != nil {
		log.Println(err)
	}
}

func (client *Client) Register(ctx context.Context, name, email, password string) (string, error) {
	response, err := client.service.Register(ctx, &pb.RegisterRequest{
		Name:     name,
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", err
	}
	return response.Value, nil
}

func (client *Client) Login(ctx context.Context, email, password string) (string, error) {
	response, err := client.service.Login(ctx, &pb.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", err
	}
	return response.Value, nil
}

func (client *Client) GetAccount(ctx context.Context, Id uint64) (*models.Account, error) {
	r, err := client.service.GetAccount(
		ctx,
		&wrapperspb.UInt64Value{
			Value: Id,
		},
	)
	if err != nil {
		return nil, err
	}
	return &models.Account{
		ID:    r.Account.GetId(),
		Name:  r.Account.GetName(),
		Email: r.Account.GetEmail(),
	}, nil
}

func (client *Client) GetAccounts(ctx context.Context, skip, take uint64) ([]models.Account, error) {
	r, err := client.service.GetAccounts(
		ctx,
		&pb.GetAccountsRequest{Take: take, Skip: skip},
	)
	if err != nil {
		return nil, err
	}
	var accounts []models.Account
	for _, a := range r.Accounts {
		accounts = append(accounts, models.Account{
			ID:    a.GetId(),
			Name:  a.GetName(),
			Email: a.GetEmail(),
		})
	}
	return accounts, nil
}


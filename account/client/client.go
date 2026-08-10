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

func (client *Client) RequestPasswordReset(ctx context.Context, email string) error {
	_, err := client.service.ForgotPassword(ctx, &pb.ForgotPasswordRequest{Email: email})
	return err
}

func (client *Client) ResetPassword(ctx context.Context, email, otp, newPassword string) error {
	_, err := client.service.ResetPassword(ctx, &pb.ResetPasswordRequest{Email: email, Otp: otp, NewPassword: newPassword})
	return err
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

func (client *Client) LoginWithGoogle(ctx context.Context, credential string) (string, error) {
	response, err := client.service.LoginWithGoogle(ctx, &pb.GoogleLoginRequest{Credential: credential})
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
		ID: r.Account.GetId(), Name: r.Account.GetName(), Email: r.Account.GetEmail(), RoleID: int(r.Account.GetRoleId()), Role: r.Account.GetRole(), Status: r.Account.GetStatus(),
		AvatarURL: r.Account.GetAvatarUrl(), Phone: r.Account.GetPhone(), ShippingAddress: r.Account.GetShippingAddress(),
	}, nil
}

func (client *Client) UpdateProfile(ctx context.Context, id uint64, name, email, avatarURL, phone, shippingAddress string) (*models.Account, error) {
	r, err := client.service.UpdateProfile(ctx, &pb.UpdateProfileRequest{AccountId: id, Name: name, Email: email, AvatarUrl: avatarURL, Phone: phone, ShippingAddress: shippingAddress})
	if err != nil {
		return nil, err
	}
	return accountFromProto(r.Account), nil
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
			ID: a.GetId(), Name: a.GetName(), Email: a.GetEmail(), RoleID: int(a.GetRoleId()), Role: a.GetRole(), Status: a.GetStatus(),
			AvatarURL: a.GetAvatarUrl(), Phone: a.GetPhone(), ShippingAddress: a.GetShippingAddress(),
		})
	}
	return accounts, nil
}

func (client *Client) ResolveEmail(ctx context.Context, accountID uint64) (string, error) {
	account, err := client.GetAccount(ctx, accountID)
	if err != nil {
		return "", err
	}
	return account.Email, nil
}

func (client *Client) SetAccountStatus(ctx context.Context, actorID, targetID uint64, actorRole, status, requestID string) (*models.Account, error) {
	r, err := client.service.SetAccountStatus(ctx, &pb.SetAccountStatusRequest{ActorId: actorID, TargetId: targetID, ActorRole: actorRole, Status: status, RequestId: requestID})
	if err != nil {
		return nil, err
	}
	return accountFromProto(r.Account), nil
}

func (client *Client) SetAccountRole(ctx context.Context, actorID, targetID uint64, actorRole, role, requestID string) (*models.Account, error) {
	r, err := client.service.SetAccountRole(ctx, &pb.SetAccountRoleRequest{ActorId: actorID, TargetId: targetID, ActorRole: actorRole, Role: role, RequestId: requestID})
	if err != nil {
		return nil, err
	}
	return accountFromProto(r.Account), nil
}

func accountFromProto(a *pb.Account) *models.Account {
	return &models.Account{ID: a.GetId(), Name: a.GetName(), Email: a.GetEmail(), RoleID: int(a.GetRoleId()), Role: a.GetRole(), Status: a.GetStatus(), AvatarURL: a.GetAvatarUrl(), Phone: a.GetPhone(), ShippingAddress: a.GetShippingAddress()}
}

package internal

import (
	"context"
	"fmt"
	"net"

	"github.com/Tuananh165art/GoshopX/account/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type grpcServer struct {
	pb.UnimplementedAccountServiceServer
	service Service
}

func ListenGRPC(service Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	serv := grpc.NewServer(observability.GRPCServerOptions("account")...)

	pb.RegisterAccountServiceServer(serv, &grpcServer{
		UnimplementedAccountServiceServer: pb.UnimplementedAccountServiceServer{},
		service:                           service})
	reflection.Register(serv)
	return serv.Serve(lis)
}

func (server *grpcServer) Register(ctx context.Context, request *pb.RegisterRequest) (*wrapperspb.StringValue, error) {
	token, err := server.service.Register(ctx, request.Name, request.Email, request.Password)
	if err != nil {
		return nil, err
	}
	return &wrapperspb.StringValue{
		Value: token,
	}, nil
}

func (server *grpcServer) ForgotPassword(ctx context.Context, request *pb.ForgotPasswordRequest) (*wrapperspb.BoolValue, error) {
	if err := server.service.RequestPasswordReset(ctx, request.Email); err != nil {
		return nil, err
	}
	return wrapperspb.Bool(true), nil
}

func (server *grpcServer) ResetPassword(ctx context.Context, request *pb.ResetPasswordRequest) (*wrapperspb.BoolValue, error) {
	if err := server.service.ResetPassword(ctx, request.Email, request.Otp, request.NewPassword); err != nil {
		return nil, err
	}
	return wrapperspb.Bool(true), nil
}

func (server *grpcServer) Login(ctx context.Context, request *pb.LoginRequest) (*wrapperspb.StringValue, error) {
	token, err := server.service.Login(ctx, request.Email, request.Password)
	if err != nil {
		return nil, err
	}
	return &wrapperspb.StringValue{
		Value: token,
	}, nil
}

func (server *grpcServer) LoginWithGoogle(ctx context.Context, request *pb.GoogleLoginRequest) (*wrapperspb.StringValue, error) {
	token, err := server.service.LoginWithGoogle(ctx, request.Credential)
	if err != nil {
		return nil, err
	}
	return &wrapperspb.StringValue{Value: token}, nil
}

func (server *grpcServer) GetAccount(ctx context.Context, r *wrapperspb.UInt64Value) (*pb.AccountResponse, error) {
	a, err := server.service.GetAccount(ctx, r.Value)
	if err != nil {
		return nil, err
	}
	return &pb.AccountResponse{Account: &pb.Account{
		Id: uint64(a.ID), Name: a.Name, Email: a.Email, Role: a.Role, RoleId: int32(a.RoleID), Status: a.Status,
		AvatarUrl: a.AvatarURL, Phone: a.Phone, ShippingAddress: a.ShippingAddress,
	}}, nil
}

func (server *grpcServer) UpdateProfile(ctx context.Context, request *pb.UpdateProfileRequest) (*pb.AccountResponse, error) {
	a, err := server.service.UpdateProfile(ctx, request.AccountId, request.Name, request.Email, request.AvatarUrl, request.Phone, request.ShippingAddress)
	if err != nil {
		return nil, err
	}
	return &pb.AccountResponse{Account: &pb.Account{
		Id: a.ID, Name: a.Name, Email: a.Email, Role: a.Role, RoleId: int32(a.RoleID), Status: a.Status,
		AvatarUrl: a.AvatarURL, Phone: a.Phone, ShippingAddress: a.ShippingAddress,
	}}, nil
}

func (server *grpcServer) GetAccounts(ctx context.Context, r *pb.GetAccountsRequest) (*pb.GetAccountsResponse, error) {
	res, err := server.service.GetAccounts(ctx, r.Skip, r.Take)
	if err != nil {
		return nil, err
	}
	var accounts []*pb.Account
	for _, p := range res {
		accounts = append(accounts, &pb.Account{
			Id:        uint64(int(p.ID)),
			Name:      p.Name,
			Email:     p.Email,
			Role:      p.Role,
			Status:    p.Status,
			AvatarUrl: p.AvatarURL, Phone: p.Phone, ShippingAddress: p.ShippingAddress,
		},
		)
	}
	return &pb.GetAccountsResponse{Accounts: accounts}, nil
}

func (server *grpcServer) SetAccountStatus(ctx context.Context, request *pb.SetAccountStatusRequest) (*pb.AccountResponse, error) {
	a, err := server.service.SetAccountStatus(ctx, request.ActorId, request.TargetId, request.ActorRole, request.Status, request.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.AccountResponse{Account: &pb.Account{Id: a.ID, Name: a.Name, Email: a.Email, Role: a.Role, RoleId: int32(a.RoleID), Status: a.Status}}, nil
}

func (server *grpcServer) SetAccountRole(ctx context.Context, request *pb.SetAccountRoleRequest) (*pb.AccountResponse, error) {
	a, err := server.service.SetAccountRole(ctx, request.ActorId, request.TargetId, request.ActorRole, request.Role, request.RequestId)
	if err != nil {
		return nil, err
	}
	return &pb.AccountResponse{Account: &pb.Account{Id: a.ID, Name: a.Name, Email: a.Email, Role: a.Role, RoleId: int32(a.RoleID), Status: a.Status}}, nil
}

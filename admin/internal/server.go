package internal

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type grpcServer struct {
	pb.UnimplementedAdminServiceServer
	service Service
}

func ListenGRPC(service Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	server := grpc.NewServer(observability.GRPCServerOptions("admin")...)
	pb.RegisterAdminServiceServer(server, &grpcServer{service: service})
	reflection.Register(server)
	return server.Serve(lis)
}

func (s *grpcServer) GetDashboard(ctx context.Context, request *pb.DashboardRequest) (*pb.DashboardResponse, error) {
	dashboard, err := s.service.Dashboard(ctx, time.Unix(request.FromUnix, 0).UTC(), time.Unix(request.ToUnix, 0).UTC(), int(request.TopProductsLimit))
	if err != nil {
		return nil, err
	}
	response := &pb.DashboardResponse{Gmv: dashboard.GMV, Revenue: dashboard.Revenue, OrderCount: dashboard.OrderCount, AverageOrderValue: dashboard.AverageOrderValue, PaymentAttempts: dashboard.PaymentAttempts, SuccessfulPayments: dashboard.SuccessfulPayments, PaymentSuccessRate: dashboard.PaymentSuccessRate}
	for _, product := range dashboard.TopProducts {
		response.TopProducts = append(response.TopProducts, &pb.ProductMetric{ProductId: product.ProductID, Quantity: product.Quantity, Gmv: product.GMV})
	}
	return response, nil
}

func (s *grpcServer) ListAuditEvents(ctx context.Context, request *pb.ListAuditEventsRequest) (*pb.ListAuditEventsResponse, error) {
	items, err := s.service.ListAudit(ctx, request.ActorAccountId, request.Action, time.Unix(request.FromUnix, 0).UTC(), time.Unix(request.ToUnix, 0).UTC(), int(request.Skip), int(request.Take))
	if err != nil {
		return nil, err
	}
	response := &pb.ListAuditEventsResponse{}
	for _, item := range items {
		response.Events = append(response.Events, &pb.AuditEvent{EventId: item.EventID, ActorAccountId: item.ActorAccountID, TargetAccountId: item.TargetAccountID, Action: item.Action, Outcome: item.Outcome, RequestId: item.RequestID, OccurredAtUnix: item.OccurredAt.Unix()})
	}
	return response, nil
}

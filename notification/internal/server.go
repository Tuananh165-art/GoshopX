package internal

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/notification/models"
	"github.com/Tuananh165art/GoshopX/notification/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type grpcServer struct {
	pb.UnimplementedNotificationServiceServer
	service Service
}

func StartServers(service Service, consumer sarama.Consumer, port int) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if consumer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := NewEventConsumer(consumer, service).Start(context.Background()); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ListenGRPC(service, port); err != nil {
			errCh <- err
		}
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	return <-errCh
}

func ListenGRPC(service Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	server := grpc.NewServer(observability.GRPCServerOptions("notification")...)
	pb.RegisterNotificationServiceServer(server, &grpcServer{
		UnimplementedNotificationServiceServer: pb.UnimplementedNotificationServiceServer{},
		service:                                service,
	})
	reflection.Register(server)
	return server.Serve(lis)
}

func (server *grpcServer) ListNotifications(ctx context.Context, request *pb.ListNotificationsRequest) (*pb.ListNotificationsResponse, error) {
	notifications, err := server.service.ListNotifications(ctx, request.AccountId, request.Skip, request.Take)
	if err != nil {
		return nil, err
	}
	response := &pb.ListNotificationsResponse{Notifications: make([]*pb.Notification, 0, len(notifications))}
	for _, notification := range notifications {
		response.Notifications = append(response.Notifications, encodeNotification(notification))
	}
	return response, nil
}

func (server *grpcServer) CountUnread(ctx context.Context, request *pb.CountUnreadRequest) (*pb.CountUnreadResponse, error) {
	count, err := server.service.CountUnread(ctx, request.AccountId)
	if err != nil {
		return nil, err
	}
	return &pb.CountUnreadResponse{Count: count}, nil
}

func (server *grpcServer) MarkRead(ctx context.Context, request *pb.MarkReadRequest) (*pb.MarkReadResponse, error) {
	ok, err := server.service.MarkRead(ctx, request.AccountId, request.NotificationId)
	if err != nil {
		return nil, err
	}
	return &pb.MarkReadResponse{Ok: ok}, nil
}

func (server *grpcServer) MarkAllRead(ctx context.Context, request *pb.MarkAllReadRequest) (*pb.MarkAllReadResponse, error) {
	count, err := server.service.MarkAllRead(ctx, request.AccountId)
	if err != nil {
		return nil, err
	}
	return &pb.MarkAllReadResponse{UpdatedCount: count}, nil
}

func encodeNotification(notification *models.Notification) *pb.Notification {
	result := &pb.Notification{
		Id:            uint64(notification.ID),
		AccountId:     notification.AccountID,
		EventId:       notification.EventID,
		EventType:     notification.EventType,
		Title:         notification.Title,
		Message:       notification.Message,
		MetadataJson:  notification.MetadataJSON,
		IsRead:        notification.IsRead,
		CreatedAtUnix: notification.CreatedAt.Unix(),
	}
	if notification.ReadAt != nil {
		result.ReadAtUnix = notification.ReadAt.Unix()
	}
	return result
}

package client

import (
	"context"
	"log"
	"time"

	"github.com/Tuananh165art/GoshopX/notification/models"
	"github.com/Tuananh165art/GoshopX/notification/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.NotificationServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, service: pb.NewNotificationServiceClient(conn)}, nil
}

func (client *Client) Close() {
	if err := client.conn.Close(); err != nil {
		log.Println(err)
	}
}

func (client *Client) ListNotifications(ctx context.Context, accountID uint64, skip, take uint64) ([]*models.Notification, error) {
	response, err := client.service.ListNotifications(ctx, &pb.ListNotificationsRequest{AccountId: accountID, Skip: skip, Take: take})
	if err != nil {
		return nil, err
	}
	notifications := make([]*models.Notification, 0, len(response.Notifications))
	for _, notification := range response.Notifications {
		notifications = append(notifications, &models.Notification{
			ID:           uint(notification.Id),
			AccountID:    notification.AccountId,
			EventID:      notification.EventId,
			EventType:    notification.EventType,
			Title:        notification.Title,
			Message:      notification.Message,
			MetadataJSON: notification.MetadataJson,
			IsRead:       notification.IsRead,
			CreatedAt:    time.Unix(notification.CreatedAtUnix, 0).UTC(),
		})
	}
	return notifications, nil
}

func (client *Client) CountUnread(ctx context.Context, accountID uint64) (int64, error) {
	response, err := client.service.CountUnread(ctx, &pb.CountUnreadRequest{AccountId: accountID})
	if err != nil {
		return 0, err
	}
	return response.Count, nil
}

func (client *Client) MarkRead(ctx context.Context, accountID, notificationID uint64) (bool, error) {
	response, err := client.service.MarkRead(ctx, &pb.MarkReadRequest{AccountId: accountID, NotificationId: notificationID})
	if err != nil {
		return false, err
	}
	return response.Ok, nil
}

func (client *Client) MarkAllRead(ctx context.Context, accountID uint64) (int64, error) {
	response, err := client.service.MarkAllRead(ctx, &pb.MarkAllReadRequest{AccountId: accountID})
	if err != nil {
		return 0, err
	}
	return response.UpdatedCount, nil
}


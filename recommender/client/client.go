package client

import (
	"context"

	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"github.com/Tuananh165art/GoshopX/recommender/generated/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.RecommenderServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, observability.GRPCClientOptions()...)...)
	if err != nil {
		return nil, err
	}
	client := pb.NewRecommenderServiceClient(conn)
	return &Client{conn, client}, nil
}

func (client *Client) Close() {
	client.conn.Close()
}

func (client *Client) GetRecommendationForUser(ctx context.Context, userId string, skip, take uint64) (*pb.RecommendationResponse, error) {
	return client.service.GetRecommendations(ctx, &pb.RecommendationRequestForUserId{UserId: userId, Skip: skip, Take: take})
}

func (client *Client) GetRecommendationBasedOnViewed(ctx context.Context, ids []string, skip, take uint64) (*pb.RecommendationResponse, error) {
	return client.service.GetRecommendationsBasedOnViewed(ctx, &pb.RecommendationRequestOnViews{Ids: ids, Skip: skip, Take: take})
}

func (client *Client) ChatRecommend(ctx context.Context, text, imageURL, userID, sessionID string, viewedIDs []string, limit uint32, requestID string) (*pb.ChatRecommendationResponse, error) {
	return client.service.ChatRecommend(ctx, &pb.ChatRecommendationRequest{
		Text:             text,
		ImageUrl:         imageURL,
		UserId:           userID,
		SessionId:        sessionID,
		ViewedProductIds: viewedIDs,
		Limit:            limit,
		RequestId:        requestID,
	})
}

func (client *Client) DeleteChatHistory(ctx context.Context, userID, sessionID string) (bool, error) {
	response, err := client.service.DeleteChatHistory(ctx, &pb.DeleteChatHistoryRequest{UserId: userID, SessionId: sessionID})
	if err != nil {
		return false, err
	}
	return response.Deleted, nil
}

func (client *Client) EnqueueKnowledgeIngestion(ctx context.Context, sourceID, name, objectURI, contentType, sha256 string, ownerAccountID int64, idempotencyKey, linkedProductID string) (*pb.KnowledgeIngestionJob, error) {
	return client.service.EnqueueKnowledgeIngestion(ctx, &pb.EnqueueKnowledgeIngestionRequest{
		SourceId: sourceID, Name: name, ObjectUri: objectURI, ContentType: contentType,
		Sha256: sha256, OwnerAccountId: ownerAccountID, IdempotencyKey: idempotencyKey, LinkedProductId: linkedProductID,
	})
}

func (client *Client) GetKnowledgeIngestionJob(ctx context.Context, jobID string) (*pb.KnowledgeIngestionJob, error) {
	return client.service.GetKnowledgeIngestionJob(ctx, &pb.KnowledgeIngestionJobRequest{JobId: jobID})
}

func (client *Client) ListKnowledgeIngestionJobs(ctx context.Context, limit uint32) (*pb.KnowledgeIngestionJobList, error) {
	return client.service.ListKnowledgeIngestionJobs(ctx, &pb.ListKnowledgeIngestionJobsRequest{Limit: limit})
}

func (client *Client) CancelKnowledgeIngestionJob(ctx context.Context, jobID string, ownerAccountID int64) (bool, error) {
	response, err := client.service.CancelKnowledgeIngestionJob(ctx, &pb.CancelKnowledgeIngestionJobRequest{JobId: jobID, OwnerAccountId: ownerAccountID})
	if err != nil {
		return false, err
	}
	return response.Cancelled, nil
}

func (client *Client) RetryKnowledgeIngestionJob(ctx context.Context, jobID string, actorAccountID int64) (bool, error) {
	response, err := client.service.RetryKnowledgeIngestionJob(ctx, &pb.RetryKnowledgeIngestionJobRequest{JobId: jobID, ActorAccountId: actorAccountID})
	if err != nil {
		return false, err
	}
	return response.Queued, nil
}

func (client *Client) SetKnowledgeSourceStatus(ctx context.Context, sourceID, status string, actorAccountID int64) (bool, error) {
	response, err := client.service.SetKnowledgeSourceStatus(ctx, &pb.SetKnowledgeSourceStatusRequest{SourceId: sourceID, Status: status, ActorAccountId: actorAccountID})
	if err != nil {
		return false, err
	}
	return response.Updated, nil
}

func (client *Client) ListIngestionAuditEvents(ctx context.Context, limit uint32) (*pb.IngestionAuditEventList, error) {
	return client.service.ListIngestionAuditEvents(ctx, &pb.ListIngestionAuditEventsRequest{Limit: limit})
}

func (client *Client) PreviewKnowledgeDocument(ctx context.Context, name, contentType string, content []byte) (*pb.KnowledgeDocumentPreview, error) {
	return client.service.PreviewKnowledgeDocument(ctx, &pb.PreviewKnowledgeDocumentRequest{
		Name: name, ContentType: contentType, Content: content,
	}, grpc.MaxCallSendMsgSize(21<<20))
}

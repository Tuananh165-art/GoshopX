package client

import (
	"context"
	"log"

	"github.com/Tuananh165art/GoshopX/product/models"
	"github.com/Tuananh165art/GoshopX/product/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.ProductServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := pb.NewProductServiceClient(conn)
	return &Client{conn, client}, nil
}

func (client *Client) Close() {
	err := client.conn.Close()
	if err != nil {
		log.Println(err)
	}
}

func (client *Client) GetProduct(ctx context.Context, id string) (*models.Product, error) {
	res, err := client.service.GetProduct(ctx, &wrapperspb.StringValue{
		Value: id,
	})
	if err != nil {
		return nil, err
	}
	return &models.Product{
		ID:          res.Product.Id,
		Name:        res.Product.Name,
		Description: res.Product.Description,
		Price:       res.Product.Price,
		AccountID:   int(res.Product.GetAccountId()),
	}, nil
}

func (client *Client) GetProducts(ctx context.Context, skip, take uint64, ids []string, query string) ([]models.Product, error) {
	res, err := client.service.GetProducts(ctx, &pb.GetProductsRequest{
		Skip:  skip,
		Take:  take,
		Ids:   ids,
		Query: query,
	})
	if err != nil {
		return nil, err
	}
	var products []models.Product
	for _, p := range res.Products {
		products = append(products, models.Product{
			ID:          p.Id,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			AccountID:   int(p.AccountId),
		})
	}
	return products, nil
}

func (client *Client) PostProduct(ctx context.Context, name, description string, price float64, accountId int64) (*models.Product, error) {
	res, err := client.service.PostProduct(ctx, &pb.CreateProductRequest{
		Name:        name,
		Description: description,
		Price:       price,
		AccountId:   accountId,
	})
	if err != nil {
		log.Println("Error creating product", err)
		return nil, err
	}
	return &models.Product{
		ID:          res.Product.Id,
		Name:        res.Product.Name,
		Description: res.Product.Description,
		Price:       res.Product.Price,
		AccountID:   int(res.Product.GetAccountId()),
	}, nil
}

func (client *Client) UpdateProduct(ctx context.Context, id, name, description string, price float64, accountId int64) (*models.Product, error) {
	res, err := client.service.UpdateProduct(ctx, &pb.UpdateProductRequest{
		Id:          id,
		Name:        name,
		Description: description,
		Price:       price,
		AccountId:   accountId,
	})
	if err != nil {
		return nil, err
	}
	return &models.Product{
		ID:          res.Product.Id,
		Name:        res.Product.Name,
		Description: res.Product.Description,
		Price:       res.Product.Price,
		AccountID:   int(res.Product.GetAccountId()),
	}, nil
}

func (client *Client) DeleteProduct(ctx context.Context, productId string, accountId int64) error {
	_, err := client.service.DeleteProduct(ctx, &pb.DeleteProductRequest{ProductId: productId, AccountId: accountId})
	return err
}


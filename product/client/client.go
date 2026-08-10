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

func fromProtoProduct(p *pb.Product) *models.Product {
	media := make([]models.ProductMedia, 0, len(p.Media))
	for _, item := range p.Media {
		media = append(media, models.ProductMedia{ID: item.Id, URL: item.Url, AltText: item.AltText, SortOrder: int(item.SortOrder)})
	}
	reviews := make([]models.ProductReview, 0, len(p.Reviews))
	for _, review := range p.Reviews {
		reviews = append(reviews, models.ProductReview{Rating: int(review.Rating), Comment: review.Comment, Date: review.Date, ReviewerName: review.ReviewerName, ReviewerEmail: review.ReviewerEmail})
	}
	dimensions, meta := models.ProductDimensions{}, models.ProductMeta{}
	if p.Dimensions != nil {
		dimensions = models.ProductDimensions{Width: p.Dimensions.Width, Height: p.Dimensions.Height, Depth: p.Dimensions.Depth}
	}
	if p.Meta != nil {
		meta = models.ProductMeta{CreatedAt: p.Meta.CreatedAt, UpdatedAt: p.Meta.UpdatedAt, Barcode: p.Meta.Barcode, QRCode: p.Meta.QrCode}
	}
	return &models.Product{ID: p.Id, Name: p.Name, Description: p.Description, Price: p.Price, DiscountPercentage: p.DiscountPercentage, Rating: p.Rating, Stock: int(p.Stock), Tags: p.Tags, Brand: p.Brand, SKU: p.Sku, Weight: p.Weight, Dimensions: dimensions, WarrantyInformation: p.WarrantyInformation, ShippingInformation: p.ShippingInformation, AvailabilityStatus: p.AvailabilityStatus, Reviews: reviews, ReturnPolicy: p.ReturnPolicy, MinimumOrderQuantity: int(p.MinimumOrderQuantity), Meta: meta, Thumbnail: p.Thumbnail, Images: p.Images, AccountID: int(p.AccountId), CategoryID: p.CategoryId, PublishStatus: p.PublishStatus, ModerationStatus: p.ModerationStatus, ModerationReason: p.ModerationReason, Media: media}
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
	return fromProtoProduct(res.Product), nil
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
		products = append(products, *fromProtoProduct(p))
	}
	return products, nil
}

func (client *Client) GetAllProducts(ctx context.Context, skip, take uint64) ([]models.Product, error) {
	res, err := client.service.AdminGetProducts(ctx, &pb.AdminGetProductsRequest{Skip: skip, Take: take})
	if err != nil {
		return nil, err
	}
	products := make([]models.Product, 0, len(res.Products))
	for _, p := range res.Products {
		products = append(products, *fromProtoProduct(p))
	}
	return products, nil
}

func (client *Client) AdminUpdateProduct(ctx context.Context, product *models.Product, actorRole string) (*models.Product, error) {
	res, err := client.service.AdminUpdateProduct(ctx, &pb.AdminProductRequest{Product: &pb.Product{Id: product.ID, Name: product.Name, Description: product.Description, Price: product.Price, AccountId: int64(product.AccountID), CategoryId: product.CategoryID, PublishStatus: product.PublishStatus, ModerationStatus: product.ModerationStatus, ModerationReason: product.ModerationReason, Thumbnail: product.Thumbnail, Images: product.Images, Tags: product.Tags, Brand: product.Brand, Sku: product.SKU}, ActorRole: actorRole})
	if err != nil {
		return nil, err
	}
	return fromProtoProduct(res.Product), nil
}

func (client *Client) AdminDeleteProduct(ctx context.Context, productID, actorRole string) error {
	_, err := client.service.AdminDeleteProduct(ctx, &pb.AdminDeleteProductRequest{ProductId: productID, ActorRole: actorRole})
	return err
}

func (client *Client) GetProductsByCategory(ctx context.Context, category string, skip, take uint64) ([]models.Product, error) {
	res, err := client.service.GetProducts(ctx, &pb.GetProductsRequest{Category: category, Skip: skip, Take: take})
	if err != nil {
		return nil, err
	}
	products := make([]models.Product, 0, len(res.Products))
	for _, p := range res.Products {
		products = append(products, *fromProtoProduct(p))
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
	return fromProtoProduct(res.Product), nil
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
	return fromProtoProduct(res.Product), nil
}

func (client *Client) ModerateProduct(ctx context.Context, productID, publishStatus, moderationStatus, reason, actorRole string) (*models.Product, error) {
	res, err := client.service.ModerateProduct(ctx, &pb.ModerateProductRequest{ProductId: productID, PublishStatus: publishStatus, ModerationStatus: moderationStatus, ModerationReason: reason, ActorRole: actorRole})
	if err != nil {
		return nil, err
	}
	return fromProtoProduct(res.Product), nil
}

func (client *Client) AddReview(ctx context.Context, productID string, rating int, comment, reviewerName, reviewerEmail string) (*models.Product, error) {
	res, err := client.service.AddProductReview(ctx, &pb.AddProductReviewRequest{ProductId: productID, Rating: int32(rating), Comment: comment, ReviewerName: reviewerName, ReviewerEmail: reviewerEmail})
	if err != nil {
		return nil, err
	}
	return fromProtoProduct(res.Product), nil
}

func (client *Client) AddProductMedia(ctx context.Context, productID, url, altText, actorRole string, sortOrder int32) (*models.Product, error) {
	res, err := client.service.AddProductMedia(ctx, &pb.AddProductMediaRequest{ProductId: productID, ActorRole: actorRole, Media: &pb.ProductMedia{Url: url, AltText: altText, SortOrder: sortOrder}})
	if err != nil {
		return nil, err
	}
	return fromProtoProduct(res.Product), nil
}

func (client *Client) CreateCategory(ctx context.Context, name, slug, description, actorRole string, isActive bool) (*models.Category, error) {
	res, err := client.service.CreateCategory(ctx, &pb.CategoryRequest{ActorRole: actorRole, Category: &pb.Category{Name: name, Slug: slug, Description: description, IsActive: isActive}})
	if err != nil {
		return nil, err
	}
	return &models.Category{ID: res.Id, Name: res.Name, Slug: res.Slug, Description: res.Description, IsActive: res.IsActive}, nil
}

func (client *Client) ListCategories(ctx context.Context, activeOnly bool) ([]models.Category, error) {
	res, err := client.service.ListCategories(ctx, &pb.ListCategoriesRequest{ActiveOnly: activeOnly})
	if err != nil {
		return nil, err
	}
	items := make([]models.Category, 0, len(res.Categories))
	for _, item := range res.Categories {
		items = append(items, models.Category{ID: item.Id, Name: item.Name, Slug: item.Slug, Description: item.Description, IsActive: item.IsActive})
	}
	return items, nil
}

func (client *Client) DeleteProduct(ctx context.Context, productId string, accountId int64) error {
	_, err := client.service.DeleteProduct(ctx, &pb.DeleteProductRequest{ProductId: productId, AccountId: accountId})
	return err
}

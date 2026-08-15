package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"github.com/Tuananh165art/GoshopX/product/models"
	"github.com/Tuananh165art/GoshopX/product/proto/pb"
)

type grpcServer struct {
	pb.UnimplementedProductServiceServer
	service Service
}

func toProtoProduct(p *models.Product) *pb.Product {
	media := make([]*pb.ProductMedia, 0, len(p.Media))
	for _, item := range p.Media {
		media = append(media, &pb.ProductMedia{Id: item.ID, Url: item.URL, AltText: item.AltText, SortOrder: int32(item.SortOrder)})
	}
	reviews := make([]*pb.ProductReview, 0, len(p.Reviews))
	for _, review := range p.Reviews {
		reviews = append(reviews, &pb.ProductReview{Rating: int32(review.Rating), Comment: review.Comment, Date: review.Date, ReviewerName: review.ReviewerName, ReviewerEmail: review.ReviewerEmail})
	}
	return &pb.Product{Id: p.ID, Name: p.Name, Description: p.Description, Price: p.Price, AccountId: int64(p.AccountID), CategoryId: p.CategoryID, PublishStatus: p.PublishStatus, ModerationStatus: p.ModerationStatus, ModerationReason: p.ModerationReason, Media: media, DiscountPercentage: p.DiscountPercentage, Rating: p.Rating, Stock: int32(p.Stock), Tags: p.Tags, Brand: p.Brand, Sku: p.SKU, Weight: p.Weight, Dimensions: &pb.ProductDimensions{Width: p.Dimensions.Width, Height: p.Dimensions.Height, Depth: p.Dimensions.Depth}, WarrantyInformation: p.WarrantyInformation, ShippingInformation: p.ShippingInformation, AvailabilityStatus: p.AvailabilityStatus, Reviews: reviews, ReturnPolicy: p.ReturnPolicy, MinimumOrderQuantity: int32(p.MinimumOrderQuantity), Meta: &pb.ProductMeta{CreatedAt: p.Meta.CreatedAt, UpdatedAt: p.Meta.UpdatedAt, Barcode: p.Meta.Barcode, QrCode: p.Meta.QRCode}, Thumbnail: p.Thumbnail, Images: p.Images}
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

func ListenGRPC(s Service, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	serv := grpc.NewServer(observability.GRPCServerOptions("product")...)

	pb.RegisterProductServiceServer(serv, &grpcServer{
		UnimplementedProductServiceServer: pb.UnimplementedProductServiceServer{},
		service:                           s})
	reflection.Register(serv)
	return serv.Serve(lis)
}

func (s *grpcServer) GetProduct(ctx context.Context, r *wrapperspb.StringValue) (*pb.ProductResponse, error) {
	p, err := s.service.GetProduct(ctx, r.Value)
	if err != nil {
		return nil, err
	}
	return &pb.ProductResponse{Product: toProtoProduct(p)}, nil
}

func (s *grpcServer) GetProducts(ctx context.Context, r *pb.GetProductsRequest) (*pb.ProductsResponse, error) {
	var res []*models.Product
	var err error
	if r.Query != "" {
		res, err = s.service.SearchProducts(ctx, r.Query, r.Skip, r.Take)
	} else if r.Category != "" {
		res, err = s.service.GetProductsByCategory(ctx, r.Category, r.Skip, r.Take)
	} else if len(r.Ids) != 0 {
		res, err = s.service.GetProductsWithIDs(ctx, r.Ids)
	} else {
		res, err = s.service.GetProducts(ctx, r.Skip, r.Take)
	}
	if err != nil {
		return nil, err
	}
	var products []*pb.Product
	for _, p := range res {
		products = append(products, toProtoProduct(p))

	}
	return &pb.ProductsResponse{Products: products}, nil
}

func (s *grpcServer) AdminGetProducts(ctx context.Context, r *pb.AdminGetProductsRequest) (*pb.ProductsResponse, error) {
	service, ok := s.service.(interface {
		GetAllProducts(context.Context, uint64, uint64) ([]*models.Product, error)
	})
	if !ok {
		return nil, fmt.Errorf("admin product listing is not supported")
	}
	res, err := service.GetAllProducts(ctx, r.GetSkip(), r.GetTake())
	if err != nil {
		return nil, err
	}
	products := make([]*pb.Product, 0, len(res))
	for _, p := range res {
		products = append(products, toProtoProduct(p))
	}
	return &pb.ProductsResponse{Products: products}, nil
}

func (s *grpcServer) PostProduct(ctx context.Context, r *pb.CreateProductRequest) (*pb.ProductResponse, error) {
	p, err := s.service.PostProduct(ctx, r.GetName(), r.GetDescription(), r.Price, int(r.GetAccountId()))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &pb.ProductResponse{Product: toProtoProduct(p)}, nil
}

func (s *grpcServer) UpdateProduct(ctx context.Context, r *pb.UpdateProductRequest) (*pb.ProductResponse, error) {
	p, err := s.service.UpdateProduct(ctx, r.GetId(), r.GetName(), r.GetDescription(), r.Price, int(r.GetAccountId()))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &pb.ProductResponse{Product: toProtoProduct(p)}, nil
}

func (s *grpcServer) AdminUpdateProduct(ctx context.Context, r *pb.AdminProductRequest) (*pb.ProductResponse, error) {
	p, err := s.service.AdminUpdateProduct(ctx, fromProtoProduct(r.GetProduct()), r.GetActorRole())
	if err != nil {
		return nil, err
	}
	return &pb.ProductResponse{Product: toProtoProduct(p)}, nil
}

func (s *grpcServer) AdminDeleteProduct(ctx context.Context, r *pb.AdminDeleteProductRequest) (*emptypb.Empty, error) {
	if err := s.service.AdminDeleteProduct(ctx, r.GetProductId(), r.GetActorRole()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *grpcServer) ModerateProduct(ctx context.Context, r *pb.ModerateProductRequest) (*pb.ProductResponse, error) {
	p, err := s.service.SetModeration(ctx, r.GetProductId(), r.GetPublishStatus(), r.GetModerationStatus(), r.GetModerationReason(), r.GetActorRole())
	if err != nil {
		return nil, err
	}
	return &pb.ProductResponse{Product: toProtoProduct(p)}, nil
}

func (s *grpcServer) AddProductReview(ctx context.Context, r *pb.AddProductReviewRequest) (*pb.ProductResponse, error) {
	p, err := s.service.AddReview(ctx, r.GetProductId(), models.ProductReview{Rating: int(r.GetRating()), Comment: r.GetComment(), ReviewerName: r.GetReviewerName(), ReviewerEmail: r.GetReviewerEmail(), Date: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return nil, err
	}
	return &pb.ProductResponse{Product: toProtoProduct(p)}, nil
}

func (s *grpcServer) AddProductMedia(ctx context.Context, r *pb.AddProductMediaRequest) (*pb.ProductResponse, error) {
	p, err := s.service.AddMedia(ctx, r.GetProductId(), models.ProductMedia{ID: r.GetMedia().GetId(), URL: r.GetMedia().GetUrl(), AltText: r.GetMedia().GetAltText(), SortOrder: int(r.GetMedia().GetSortOrder())}, r.GetActorRole())
	if err != nil {
		return nil, err
	}
	return &pb.ProductResponse{Product: toProtoProduct(p)}, nil
}

func (s *grpcServer) CreateCategory(ctx context.Context, r *pb.CategoryRequest) (*pb.Category, error) {
	c := r.GetCategory()
	item, err := s.service.CreateCategory(ctx, models.Category{Name: c.GetName(), Slug: c.GetSlug(), Description: c.GetDescription(), IsActive: c.GetIsActive()}, r.GetActorRole())
	if err != nil {
		return nil, err
	}
	return &pb.Category{Id: item.ID, Name: item.Name, Slug: item.Slug, Description: item.Description, IsActive: item.IsActive}, nil
}

func (s *grpcServer) ListCategories(ctx context.Context, r *pb.ListCategoriesRequest) (*pb.CategoriesResponse, error) {
	items, err := s.service.ListCategories(ctx, r.GetActiveOnly())
	if err != nil {
		return nil, err
	}
	res := &pb.CategoriesResponse{Categories: make([]*pb.Category, 0, len(items))}
	for _, item := range items {
		res.Categories = append(res.Categories, &pb.Category{Id: item.ID, Name: item.Name, Slug: item.Slug, Description: item.Description, IsActive: item.IsActive})
	}
	return res, nil
}

func (s *grpcServer) DeleteProduct(ctx context.Context, r *pb.DeleteProductRequest) (*emptypb.Empty, error) {
	err := s.service.DeleteProduct(ctx, r.GetProductId(), int(r.GetAccountId()))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

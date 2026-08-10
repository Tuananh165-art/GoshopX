package internal

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/IBM/sarama"

	"github.com/Tuananh165art/GoshopX/pkg/kafka"
	"github.com/Tuananh165art/GoshopX/product/models"
)

type Service interface {
	PostProduct(ctx context.Context, name, description string, price float64, accountId int) (*models.Product, error)
	GetProduct(ctx context.Context, id string) (*models.Product, error)
	GetProducts(ctx context.Context, skip, take uint64) ([]*models.Product, error)
	GetProductsByCategory(ctx context.Context, category string, skip, take uint64) ([]*models.Product, error)
	GetProductsWithIDs(ctx context.Context, ids []string) ([]*models.Product, error)
	SearchProducts(ctx context.Context, query string, skip, take uint64) ([]*models.Product, error)
	UpdateProduct(ctx context.Context, id, name, description string, price float64, accountId int) (*models.Product, error)
	DeleteProduct(ctx context.Context, productId string, accountId int) error
	AdminUpdateProduct(ctx context.Context, product *models.Product, actorRole string) (*models.Product, error)
	AdminDeleteProduct(ctx context.Context, productID, actorRole string) error
	SetModeration(ctx context.Context, productID, publishStatus, moderationStatus, reason, actorRole string) (*models.Product, error)
	AddMedia(ctx context.Context, productID string, media models.ProductMedia, actorRole string) (*models.Product, error)
	AddReview(ctx context.Context, productID string, review models.ProductReview) (*models.Product, error)
	CreateCategory(ctx context.Context, category models.Category, actorRole string) (*models.Category, error)
	ListCategories(ctx context.Context, activeOnly bool) ([]*models.Category, error)
	GetProducer() sarama.AsyncProducer
}

type productService struct {
	repo     Repository
	producer sarama.AsyncProducer
}

func NewProductService(repository Repository, producer sarama.AsyncProducer) Service {
	return &productService{repository, producer}
}

func (service productService) GetProducer() sarama.AsyncProducer {
	return service.producer
}

func (service productService) PostProduct(ctx context.Context, name, description string, price float64, accountId int) (*models.Product, error) {
	product := models.Product{
		Name:             name,
		Description:      description,
		Price:            price,
		AccountID:        accountId,
		PublishStatus:    "draft",
		ModerationStatus: "pending",
	}

	err := service.repo.PutProduct(ctx, &product)
	if err != nil {
		return nil, err
	}

	go func() {
		err = kafka.SendMessageToRecommender(service, models.Event{
			Type: "product_created",
			Data: models.EventData{
				ID:          &product.ID,
				Name:        &product.Name,
				Description: &product.Description,
				Price:       &product.Price,
				AccountID:   &product.AccountID,
			},
		}, "product_events")
		if err != nil {
			log.Println("Failed to send event to recommendation service:", err)
		}
	}()

	return &product, nil
}

func isCatalogAdmin(role string) bool {
	return role == "operations_admin" || role == "platform_admin"
}

func (service productService) AdminUpdateProduct(ctx context.Context, product *models.Product, actorRole string) (*models.Product, error) {
	if !isCatalogAdmin(actorRole) {
		return nil, errors.New("unauthorized")
	}
	existing, err := service.repo.GetProductById(ctx, product.ID)
	if err != nil {
		return nil, err
	}
	if product.AccountID == 0 {
		product.AccountID = existing.AccountID
	}
	if product.PublishStatus == "" {
		product.PublishStatus = existing.PublishStatus
	}
	if product.ModerationStatus == "" {
		product.ModerationStatus = existing.ModerationStatus
	}
	if product.Media == nil {
		product.Media = existing.Media
	}
	if err := service.repo.UpdateProduct(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

func (service productService) AdminDeleteProduct(ctx context.Context, productID, actorRole string) error {
	if !isCatalogAdmin(actorRole) {
		return errors.New("unauthorized")
	}
	return service.repo.DeleteProduct(ctx, productID)
}

func (service productService) SetModeration(ctx context.Context, productID, publishStatus, moderationStatus, reason, actorRole string) (*models.Product, error) {
	if !isCatalogAdmin(actorRole) {
		return nil, errors.New("unauthorized")
	}
	p, err := service.repo.GetProductById(ctx, productID)
	if err != nil {
		return nil, err
	}
	p.PublishStatus, p.ModerationStatus, p.ModerationReason = publishStatus, moderationStatus, reason
	if p.PublishStatus == "published" && p.ModerationStatus != "approved" {
		return nil, errors.New("published products must be approved")
	}
	if err := service.repo.UpdateProduct(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (service productService) AddMedia(ctx context.Context, productID string, media models.ProductMedia, actorRole string) (*models.Product, error) {
	if !isCatalogAdmin(actorRole) {
		return nil, errors.New("unauthorized")
	}
	if media.URL == "" {
		return nil, errors.New("media url is required")
	}
	p, err := service.repo.GetProductById(ctx, productID)
	if err != nil {
		return nil, err
	}
	p.Media = append(p.Media, media)
	if err := service.repo.UpdateProduct(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (service productService) AddReview(ctx context.Context, productID string, review models.ProductReview) (*models.Product, error) {
	if review.Rating < 1 || review.Rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}
	review.Comment = strings.TrimSpace(review.Comment)
	if review.Comment == "" || len(review.Comment) > 1000 {
		return nil, errors.New("comment must contain between 1 and 1000 characters")
	}
	product, err := service.repo.GetProductById(ctx, productID)
	if err != nil {
		return nil, err
	}
	product.Reviews = append(product.Reviews, review)
	total := 0
	for _, item := range product.Reviews {
		total += item.Rating
	}
	product.Rating = float64(total) / float64(len(product.Reviews))
	if err := service.repo.UpdateProduct(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

func (service productService) CreateCategory(ctx context.Context, category models.Category, actorRole string) (*models.Category, error) {
	if !isCatalogAdmin(actorRole) {
		return nil, errors.New("unauthorized")
	}
	if category.Name == "" || category.Slug == "" {
		return nil, errors.New("category name and slug are required")
	}
	if err := service.repo.PutCategory(ctx, &category); err != nil {
		return nil, err
	}
	return &category, nil
}

func (service productService) ListCategories(ctx context.Context, activeOnly bool) ([]*models.Category, error) {
	return service.repo.ListCategories(ctx, activeOnly)
}

func (service productService) GetProduct(ctx context.Context, id string) (*models.Product, error) {
	product, err := service.repo.GetProductById(ctx, id)
	if err != nil {
		return nil, err
	}

	go func() {
		err = kafka.SendMessageToRecommender(service, models.Event{
			Type: "product_retrieved",
			Data: models.EventData{
				ID:        &product.ID,
				AccountID: &product.AccountID,
			},
		}, "interaction_events")
		if err != nil {
			log.Println("Failed to send event to recommendation service:", err)
		}
	}()

	return product, nil
}

func (service productService) GetProducts(ctx context.Context, skip, take uint64) ([]*models.Product, error) {
	return service.repo.ListProducts(ctx, skip, take)
}

func (service productService) GetAllProducts(ctx context.Context, skip, take uint64) ([]*models.Product, error) {
	if repository, ok := service.repo.(interface {
		ListAllProducts(context.Context, uint64, uint64) ([]*models.Product, error)
	}); ok {
		return repository.ListAllProducts(ctx, skip, take)
	}
	return service.repo.ListProducts(ctx, skip, take)
}

func (service productService) GetProductsByCategory(ctx context.Context, category string, skip, take uint64) ([]*models.Product, error) {
	return service.repo.ListProductsByCategory(ctx, category, skip, take)
}

func (service productService) GetProductsWithIDs(ctx context.Context, ids []string) ([]*models.Product, error) {
	return service.repo.ListProductsWithIDs(ctx, ids)
}

func (service productService) SearchProducts(ctx context.Context, query string, skip, take uint64) ([]*models.Product, error) {
	return service.repo.SearchProducts(ctx, query, skip, take)
}

func (service productService) UpdateProduct(ctx context.Context, id, name, description string, price float64, accountId int) (*models.Product, error) {
	product, err := service.repo.GetProductById(ctx, id)
	if err != nil {
		return nil, err
	}
	if product.AccountID != accountId {
		return nil, errors.New("unauthorized")
	}

	updatedProduct := &models.Product{
		ID:          id,
		Name:        name,
		Description: description,
		Price:       price,
		AccountID:   accountId,
	}
	err = service.repo.UpdateProduct(ctx, updatedProduct)
	if err != nil {
		return nil, err
	}

	go func() {
		err = kafka.SendMessageToRecommender(service, models.Event{
			Type: "product_updated",
			Data: models.EventData{
				ID:          &updatedProduct.ID,
				Name:        &updatedProduct.Name,
				Description: &updatedProduct.Description,
				Price:       &updatedProduct.Price,
				AccountID:   &updatedProduct.AccountID,
			},
		}, "product_events")
		if err != nil {
			log.Println("Failed to send event to recommendation service:", err)
		}
	}()

	return updatedProduct, nil
}
func (service productService) DeleteProduct(ctx context.Context, productId string, accountId int) error {
	product, err := service.repo.GetProductById(ctx, productId)
	if err != nil {
		return err
	}
	if product.AccountID != accountId {
		return errors.New("unauthorized")
	}

	go func() {
		err = kafka.SendMessageToRecommender(service, models.Event{
			Type: "product_deleted",
			Data: models.EventData{
				ID: &product.ID,
			},
		}, "product_events")
		if err != nil {
			log.Println("Failed to send event to recommendation service:", err)
		}
	}()

	return service.repo.DeleteProduct(ctx, productId)
}

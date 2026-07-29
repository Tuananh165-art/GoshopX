package internal

import (
	"context"
	"errors"
	"log"

	"github.com/IBM/sarama"

	"github.com/rasadov/EcommerceAPI/pkg/kafka"
	"github.com/rasadov/EcommerceAPI/product/models"
)

type Service interface {
	PostProduct(ctx context.Context, name, description string, price float64, accountId int) (*models.Product, error)
	GetProduct(ctx context.Context, id string) (*models.Product, error)
	GetProducts(ctx context.Context, skip, take uint64) ([]*models.Product, error)
	GetProductsWithIDs(ctx context.Context, ids []string) ([]*models.Product, error)
	SearchProducts(ctx context.Context, query string, skip, take uint64) ([]*models.Product, error)
	UpdateProduct(ctx context.Context, id, name, description string, price float64, accountId int) (*models.Product, error)
	DeleteProduct(ctx context.Context, productId string, accountId int) error
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
		Name:        name,
		Description: description,
		Price:       price,
		AccountID:   accountId,
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

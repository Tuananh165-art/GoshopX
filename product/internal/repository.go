package internal

import (
	"context"
	"encoding/json"
	"errors"

	"gopkg.in/olivere/elastic.v5"

	"github.com/Tuananh165art/GoshopX/product/models"
)

var ErrNotFound = errors.New("entity not found")

const (
	productIndex  = "catalog"
	categoryIndex = "catalog_categories"
)

type Repository interface {
	Close()
	PutProduct(context.Context, *models.Product) error
	PutProductWithID(context.Context, *models.Product) error
	GetProductById(context.Context, string) (*models.Product, error)
	ListProducts(context.Context, uint64, uint64) ([]*models.Product, error)
	ListAllProducts(context.Context, uint64, uint64) ([]*models.Product, error)
	ListProductsByCategory(context.Context, string, uint64, uint64) ([]*models.Product, error)
	ListProductsWithIDs(context.Context, []string) ([]*models.Product, error)
	SearchProducts(context.Context, string, uint64, uint64) ([]*models.Product, error)
	UpdateProduct(context.Context, *models.Product) error
	DeleteProduct(context.Context, string) error
	PutCategory(context.Context, *models.Category) error
	ListCategories(context.Context, bool) ([]*models.Category, error)
	DeleteCategory(context.Context, string) error
}

type elasticRepository struct{ client *elastic.Client }

func NewElasticRepository(url string) (Repository, error) {
	client, err := elastic.NewClient(elastic.SetURL(url), elastic.SetSniff(false))
	if err != nil {
		return nil, err
	}
	return &elasticRepository{client: client}, nil
}

func (r *elasticRepository) Close() { r.client.Stop() }

func (r *elasticRepository) PutProduct(ctx context.Context, p *models.Product) error {
	res, err := r.client.Index().Index(productIndex).Type("product").BodyJson(p).Do(ctx)
	if err != nil {
		return err
	}
	p.ID = res.Id
	return nil
}

func (r *elasticRepository) PutProductWithID(ctx context.Context, p *models.Product) error {
	if p.ID == "" {
		return errors.New("product id is required")
	}
	_, err := r.client.Index().Index(productIndex).Type("product").Id(p.ID).BodyJson(p).Do(ctx)
	return err
}

func (r *elasticRepository) GetProductById(ctx context.Context, id string) (*models.Product, error) {
	res, err := r.client.Get().Index(productIndex).Type("product").Id(id).Do(ctx)
	if err != nil {
		return nil, err
	}
	if !res.Found {
		return nil, ErrNotFound
	}
	p := &models.Product{ID: id}
	if err := json.Unmarshal(*res.Source, p); err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

func decodeProducts(hits []*elastic.SearchHit) ([]*models.Product, error) {
	products := make([]*models.Product, 0, len(hits))
	for _, hit := range hits {
		p := &models.Product{ID: hit.Id}
		if err := json.Unmarshal(*hit.Source, p); err != nil {
			return nil, err
		}
		p.ID = hit.Id
		products = append(products, p)
	}
	return products, nil
}

func (r *elasticRepository) ListProducts(ctx context.Context, skip, take uint64) ([]*models.Product, error) {
	query := elastic.NewBoolQuery().Filter(elastic.NewTermQuery("publishStatus", "published"), elastic.NewTermQuery("moderationStatus", "approved"))
	res, err := r.client.Search().Index(productIndex).Type("product").Query(query).From(int(skip)).Size(int(take)).Do(ctx)
	if err != nil {
		return nil, err
	}
	return decodeProducts(res.Hits.Hits)
}

func (r *elasticRepository) ListAllProducts(ctx context.Context, skip, take uint64) ([]*models.Product, error) {
	res, err := r.client.Search().Index(productIndex).Type("product").From(int(skip)).Size(int(take)).Do(ctx)
	if err != nil {
		return nil, err
	}
	return decodeProducts(res.Hits.Hits)
}

func (r *elasticRepository) ListProductsByCategory(ctx context.Context, category string, skip, take uint64) ([]*models.Product, error) {
	query := elastic.NewBoolQuery().Filter(elastic.NewTermQuery("publishStatus", "published"), elastic.NewTermQuery("moderationStatus", "approved"), elastic.NewTermQuery("categoryId.keyword", category))
	res, err := r.client.Search().Index(productIndex).Type("product").Query(query).From(int(skip)).Size(int(take)).Do(ctx)
	if err != nil {
		return nil, err
	}
	return decodeProducts(res.Hits.Hits)
}

func (r *elasticRepository) ListProductsWithIDs(ctx context.Context, ids []string) ([]*models.Product, error) {
	if len(ids) == 0 {
		return []*models.Product{}, nil
	}
	items := make([]*elastic.MultiGetItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, elastic.NewMultiGetItem().Index(productIndex).Type("product").Id(id))
	}
	res, err := r.client.MultiGet().Add(items...).Do(ctx)
	if err != nil {
		return nil, err
	}
	products := make([]*models.Product, 0, len(res.Docs))
	for _, doc := range res.Docs {
		if !doc.Found {
			continue
		}
		p := &models.Product{ID: doc.Id}
		if err := json.Unmarshal(*doc.Source, p); err != nil {
			return nil, err
		}
		p.ID = doc.Id
		products = append(products, p)
	}
	return products, nil
}

func (r *elasticRepository) SearchProducts(ctx context.Context, q string, skip, take uint64) ([]*models.Product, error) {
	query := elastic.NewBoolQuery().Must(elastic.NewMultiMatchQuery(q, "name", "description", "brand", "sku", "categoryId", "tags")).Filter(elastic.NewTermQuery("publishStatus", "published"), elastic.NewTermQuery("moderationStatus", "approved"))
	res, err := r.client.Search().Index(productIndex).Type("product").Query(query).From(int(skip)).Size(int(take)).Do(ctx)
	if err != nil {
		return nil, err
	}
	return decodeProducts(res.Hits.Hits)
}

func (r *elasticRepository) UpdateProduct(ctx context.Context, p *models.Product) error {
	_, err := r.client.Update().Index(productIndex).Type("product").Id(p.ID).Doc(p).Do(ctx)
	return err
}

func (r *elasticRepository) DeleteProduct(ctx context.Context, id string) error {
	_, err := r.client.Delete().Index(productIndex).Type("product").Id(id).Do(ctx)
	return err
}

func (r *elasticRepository) PutCategory(ctx context.Context, category *models.Category) error {
	res, err := r.client.Index().Index(categoryIndex).Type("category").Id(category.ID).BodyJson(category).Do(ctx)
	if err != nil {
		return err
	}
	category.ID = res.Id
	return nil
}

func (r *elasticRepository) ListCategories(ctx context.Context, activeOnly bool) ([]*models.Category, error) {
	var q elastic.Query = elastic.NewMatchAllQuery()
	if activeOnly {
		q = elastic.NewTermQuery("isActive", true)
	}
	res, err := r.client.Search().Index(categoryIndex).Type("category").Query(q).Size(500).Do(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]*models.Category, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		item := &models.Category{ID: hit.Id}
		if err := json.Unmarshal(*hit.Source, item); err != nil {
			return nil, err
		}
		item.ID = hit.Id
		items = append(items, item)
	}
	return items, nil
}

func (r *elasticRepository) DeleteCategory(ctx context.Context, id string) error {
	_, err := r.client.Delete().Index(categoryIndex).Type("category").Id(id).Do(ctx)
	return err
}

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Tuananh165art/GoshopX/product/models"
)

type DummyJSONProduct struct {
	ID                   int                      `json:"id"`
	Title                string                   `json:"title"`
	Description          string                   `json:"description"`
	Category             string                   `json:"category"`
	Price                float64                  `json:"price"`
	DiscountPercentage   float64                  `json:"discountPercentage"`
	Rating               float64                  `json:"rating"`
	Stock                int                      `json:"stock"`
	Tags                 []string                 `json:"tags"`
	Brand                string                   `json:"brand"`
	SKU                  string                   `json:"sku"`
	Weight               float64                  `json:"weight"`
	Dimensions           models.ProductDimensions `json:"dimensions"`
	WarrantyInformation  string                   `json:"warrantyInformation"`
	ShippingInformation  string                   `json:"shippingInformation"`
	AvailabilityStatus   string                   `json:"availabilityStatus"`
	Reviews              []models.ProductReview   `json:"reviews"`
	ReturnPolicy         string                   `json:"returnPolicy"`
	MinimumOrderQuantity int                      `json:"minimumOrderQuantity"`
	Meta                 models.ProductMeta       `json:"meta"`
	Images               []string                 `json:"images"`
	Thumbnail            string                   `json:"thumbnail"`
}

type DummyJSONProductsResponse struct {
	Products []DummyJSONProduct `json:"products"`
	Total    int                `json:"total"`
	Skip     int                `json:"skip"`
	Limit    int                `json:"limit"`
}

func (p DummyJSONProduct) Model() *models.Product {
	media := make([]models.ProductMedia, 0, len(p.Images))
	for i, image := range p.Images {
		media = append(media, models.ProductMedia{ID: fmt.Sprintf("%d-%d", p.ID, i), URL: image, AltText: p.Title, SortOrder: i})
	}
	return &models.Product{
		ID: strconv.Itoa(p.ID), Name: p.Title, Description: p.Description, Price: p.Price,
		DiscountPercentage: p.DiscountPercentage, Rating: p.Rating, Stock: p.Stock, Tags: p.Tags,
		Brand: p.Brand, SKU: p.SKU, Weight: p.Weight, Dimensions: p.Dimensions,
		WarrantyInformation: p.WarrantyInformation, ShippingInformation: p.ShippingInformation,
		AvailabilityStatus: p.AvailabilityStatus, Reviews: p.Reviews, ReturnPolicy: p.ReturnPolicy,
		MinimumOrderQuantity: p.MinimumOrderQuantity, Meta: p.Meta, Thumbnail: p.Thumbnail, Images: p.Images,
		CategoryID: p.Category, PublishStatus: "published", ModerationStatus: "approved", Media: media,
	}
}

type DummyJSONClient struct {
	baseURL string
	http    *http.Client
}

func NewDummyJSONClient(baseURL string, httpClient *http.Client) *DummyJSONClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://dummyjson.com"
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &DummyJSONClient{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

func (c *DummyJSONClient) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("dummyjson returned HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(result)
}

func (c *DummyJSONClient) List(ctx context.Context, skip, limit uint64, category string) (*DummyJSONProductsResponse, error) {
	if limit == 0 {
		limit = 30
	}
	path := "/products"
	if category != "" {
		path += "/category/" + url.PathEscape(category)
	}
	path += "?limit=" + strconv.FormatUint(limit, 10) + "&skip=" + strconv.FormatUint(skip, 10)
	result := new(DummyJSONProductsResponse)
	return result, c.get(ctx, path, result)
}

func (c *DummyJSONClient) Search(ctx context.Context, query string, skip, limit uint64) (*DummyJSONProductsResponse, error) {
	if limit == 0 {
		limit = 30
	}
	result := new(DummyJSONProductsResponse)
	path := "/products/search?q=" + url.QueryEscape(query) + "&limit=" + strconv.FormatUint(limit, 10) + "&skip=" + strconv.FormatUint(skip, 10)
	return result, c.get(ctx, path, result)
}

func (c *DummyJSONClient) Get(ctx context.Context, id string) (*DummyJSONProduct, error) {
	result := new(DummyJSONProduct)
	return result, c.get(ctx, "/products/"+url.PathEscape(id), result)
}

func (c *DummyJSONClient) Seed(ctx context.Context, repository Repository, limit uint64) (int, error) {
	const pageSize = 100
	imported, skip := uint64(0), uint64(0)
	categories := make(map[string]*models.Category)
	for {
		pageLimit := uint64(pageSize)
		if limit > 0 && limit-imported < pageLimit {
			pageLimit = limit - imported
		}
		if pageLimit == 0 {
			return int(imported), nil
		}
		page, err := c.List(ctx, skip, pageLimit, "")
		if err != nil {
			return int(imported), err
		}
		for _, item := range page.Products {
			if err := repository.PutProductWithID(ctx, item.Model()); err != nil {
				return int(imported), err
			}
			if _, exists := categories[item.Category]; !exists {
				categories[item.Category] = &models.Category{ID: item.Category, Name: strings.ReplaceAll(item.Category, "-", " "), Slug: item.Category, Description: "Danh mục sản phẩm GoshopX", IsActive: true}
			}
			imported++
		}
		if len(page.Products) == 0 || imported >= uint64(page.Total) || (limit > 0 && imported >= limit) {
			for _, category := range categories {
				if err := repository.PutCategory(ctx, category); err != nil {
					return int(imported), err
				}
			}
			return int(imported), nil
		}
		skip += uint64(len(page.Products))
	}
}

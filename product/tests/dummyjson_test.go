package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tuananh165art/GoshopX/product/internal"
	"github.com/Tuananh165art/GoshopX/product/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDummyJSONProductModelPreservesCatalogFields(t *testing.T) {
	item := internal.DummyJSONProduct{
		ID: 1, Title: "Phone", Category: "smartphones", Price: 99.5, DiscountPercentage: 10,
		Rating: 4.5, Stock: 7, Brand: "Acme", SKU: "ACM-1", Images: []string{"a", "b"},
	}
	product := item.Model()

	assert.Equal(t, "1", product.ID)
	assert.Equal(t, "Phone", product.Name)
	assert.Equal(t, "smartphones", product.CategoryID)
	assert.Equal(t, 99.5, product.Price)
	assert.Equal(t, 10.0, product.DiscountPercentage)
	assert.Equal(t, 4.5, product.Rating)
	assert.Equal(t, 7, product.Stock)
	assert.Equal(t, "ACM-1", product.SKU)
	assert.Equal(t, "published", product.PublishStatus)
	assert.Equal(t, "approved", product.ModerationStatus)
	assert.Len(t, product.Media, 2)
}

func TestDummyJSONClientListAndSeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/products", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"products":[{"id":1,"title":"Phone","description":"desc","category":"smartphones","price":10,"images":["img"],"thumbnail":"thumb"}],"total":1,"skip":0,"limit":100}`))
	}))
	defer server.Close()

	client := internal.NewDummyJSONClient(server.URL, server.Client())
	page, err := client.List(context.Background(), 0, 20, "")
	assert.NoError(t, err)
	assert.Len(t, page.Products, 1)

	repository := new(MockRepository)
	repository.On("PutProductWithID", mock.Anything, mock.MatchedBy(func(product any) bool {
		item := product.(*models.Product)
		return item.ID == "1" && item.Price == 250000
	})).Return(nil).Once()
	repository.On("PutCategory", mock.Anything, mock.MatchedBy(func(category any) bool {
		item := category.(*models.Category)
		return item.ID == "smartphones" && item.Slug == "smartphones" && item.IsActive
	})).Return(nil).Once()
	seeded, err := client.WithPriceVNDExchange(25000).Seed(context.Background(), repository, 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, seeded)
	repository.AssertExpectations(t)
}

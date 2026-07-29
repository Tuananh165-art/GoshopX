package graph

import (
	"context"
	"errors"
	"log"
	"time"

	cartmodels "github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/Tuananh165art/GoshopX/graphql/generated"
	"github.com/Tuananh165art/GoshopX/graphql/utils"
	"github.com/Tuananh165art/GoshopX/pkg/auth"
	productmodels "github.com/Tuananh165art/GoshopX/product/models"
)

type queryResolver struct {
	server *Server
}

func (resolver *queryResolver) Accounts(ctx context.Context, pagination *generated.PaginationInput, id *int) ([]*generated.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if id != nil {
		res, err := resolver.server.accountClient.GetAccount(ctx, uint64(*id))
		if err != nil {
			return nil, err
		}
		return []*generated.Account{{
			ID:    int(res.ID),
			Name:  res.Name,
			Email: res.Email,
		}}, nil
	}

	skip, take := uint64(0), uint64(0)
	if pagination != nil {
		skip, take = utils.Bounds(pagination)
	}
	accountList, err := resolver.server.accountClient.GetAccounts(ctx, skip, take)
	if err != nil {
		return nil, err
	}

	var accounts []*generated.Account
	for _, account := range accountList {
		accounts = append(accounts, &generated.Account{
			ID:    int(account.ID),
			Name:  account.Name,
			Email: account.Email,
		})
	}
	return accounts, nil
}

func (resolver *queryResolver) Product(ctx context.Context, pagination *generated.PaginationInput, query, id *string, viewedProductsIds []*string, byAccountID *bool) ([]*generated.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if id != nil {
		res, err := resolver.server.productClient.GetProduct(ctx, *id)
		if err != nil {
			return nil, err
		}
		return []*generated.Product{toGeneratedProduct(res)}, nil
	}

	skip, take := uint64(0), uint64(0)
	if pagination != nil {
		skip, take = utils.Bounds(pagination)
	}

	if viewedProductsIds != nil {
		productIDs := make([]string, len(viewedProductsIds))
		for i, productID := range viewedProductsIds {
			productIDs[i] = *productID
		}
		res, err := resolver.server.recommenderClient.GetRecommendationBasedOnViewed(ctx, productIDs, skip, take)
		if err != nil {
			return nil, err
		}
		var products []*generated.Product
		for _, product := range res.GetRecommendedProducts() {
			products = append(products, &generated.Product{
				ID:          product.Id,
				Name:        product.Name,
				Description: product.Description,
				Price:       product.Price,
			})
		}
		return products, nil
	}

	if byAccountID != nil && *byAccountID {
		accountID := auth.GetUserId(ctx, true)
		if accountID == "" {
			return nil, errors.New("unauthorized")
		}
		res, err := resolver.server.recommenderClient.GetRecommendationForUser(ctx, accountID, 0, 100)
		if err != nil {
			return nil, err
		}
		var products []*generated.Product
		for _, product := range res.GetRecommendedProducts() {
			products = append(products, &generated.Product{
				ID:          product.Id,
				Name:        product.Name,
				Description: product.Description,
				Price:       product.Price,
			})
		}
		return products, nil
	}

	q := ""
	if query != nil {
		q = *query
	}
	productList, err := resolver.server.productClient.GetProducts(ctx, skip, take, nil, q)
	if err != nil {
		return nil, err
	}

	var products []*generated.Product
	for _, product := range productList {
		productCopy := product
		products = append(products, toGeneratedProduct(&productCopy))
	}
	return products, nil
}

func (resolver *queryResolver) MyCart(ctx context.Context) (*generated.Cart, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}

	cartSnapshot, err := resolver.server.cartClient.GetCart(ctx, uint64(accountID))
	if err != nil {
		return nil, err
	}
	return buildCart(ctx, resolver.server, cartSnapshot)
}

func (resolver *queryResolver) Notifications(ctx context.Context, pagination *generated.PaginationInput) ([]*generated.Notification, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}

	skip, take := uint64(0), uint64(20)
	if pagination != nil {
		skip, take = utils.Bounds(pagination)
	}
	list, err := resolver.server.notificationClient.ListNotifications(ctx, uint64(accountID), skip, take)
	if err != nil {
		return nil, err
	}

	result := make([]*generated.Notification, 0, len(list))
	for _, notification := range list {
		result = append(result, &generated.Notification{
			ID:           int(notification.ID),
			EventType:    notification.EventType,
			Title:        notification.Title,
			Message:      notification.Message,
			MetadataJSON: notification.MetadataJSON,
			IsRead:       notification.IsRead,
			CreatedAt:    notification.CreatedAt,
			ReadAt:       notification.ReadAt,
		})
	}
	return result, nil
}

func (resolver *queryResolver) UnreadNotificationCount(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return 0, err
	}
	count, err := resolver.server.notificationClient.CountUnread(ctx, uint64(accountID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func buildCart(ctx context.Context, server *Server, cartSnapshot *cartmodels.Cart) (*generated.Cart, error) {
	productIDs := make([]string, 0, len(cartSnapshot.Items))
	for _, item := range cartSnapshot.Items {
		productIDs = append(productIDs, item.ProductID)
	}
	products, err := server.productClient.GetProducts(ctx, 0, 0, productIDs, "")
	if err != nil && len(productIDs) > 0 {
		log.Println(err)
		return nil, err
	}

	productByID := map[string]*generated.Product{}
	for _, product := range products {
		productCopy := product
		productByID[product.ID] = toGeneratedProduct(&productCopy)
	}

	result := &generated.Cart{
		AccountID: int(cartSnapshot.AccountID),
		ExpiresAt: cartSnapshot.ExpiresAt,
		Items:     make([]*generated.CartItem, 0, len(cartSnapshot.Items)),
	}

	for _, item := range cartSnapshot.Items {
		product, ok := productByID[item.ProductID]
		if !ok {
			continue
		}
		result.Items = append(result.Items, &generated.CartItem{
			Product:       product,
			Quantity:      int(item.Quantity),
			ReservationID: item.ReservationID,
			ReservedUntil: item.ReservedUntil,
		})
	}
	return result, nil
}

func toGeneratedProduct(product *productmodels.Product) *generated.Product {
	return &generated.Product{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		AccountID:   product.AccountID,
	}
}


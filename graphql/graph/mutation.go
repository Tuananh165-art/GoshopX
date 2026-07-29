package graph

import (
	"context"
	"errors"
	"time"

	"github.com/Tuananh165art/GoshopX/graphql/generated"
	inventorymodels "github.com/Tuananh165art/GoshopX/inventory/models"
	ordermodels "github.com/Tuananh165art/GoshopX/order/models"
	paymentpb "github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/auth"
	"github.com/Tuananh165art/GoshopX/pkg/middleware"
)

var ErrInvalidParameter = errors.New("invalid parameter")

type mutationResolver struct {
	server *Server
}

func (resolver *mutationResolver) Register(ctx context.Context, in generated.RegisterInput) (*generated.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	token, err := resolver.server.accountClient.Register(ctx, in.Name, in.Email, in.Password)
	if err != nil {
		return nil, err
	}

	ginContext, ok := middleware.GinContextFromContext(ctx)
	if !ok {
		return nil, errors.New("could not retrieve gin context")
	}
	ginContext.SetCookie("token", token, 3600, "/", "localhost", false, true)
	return &generated.AuthResponse{Token: token}, nil
}

func (resolver *mutationResolver) Login(ctx context.Context, in generated.LoginInput) (*generated.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	token, err := resolver.server.accountClient.Login(ctx, in.Email, in.Password)
	if err != nil {
		return nil, err
	}

	ginContext, ok := middleware.GinContextFromContext(ctx)
	if !ok {
		return nil, errors.New("could not retrieve gin context")
	}
	ginContext.SetCookie("token", token, 3600, "/", "localhost", false, true)
	return &generated.AuthResponse{Token: token}, nil
}

func (resolver *mutationResolver) CreateProduct(ctx context.Context, in generated.CreateProductInput) (*generated.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	postProduct, err := resolver.server.productClient.PostProduct(ctx, in.Name, in.Description, in.Price, int64(accountID))
	if err != nil {
		return nil, err
	}
	return toGeneratedProduct(postProduct), nil
}

func (resolver *mutationResolver) UpdateProduct(ctx context.Context, in generated.UpdateProductInput) (*generated.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	product, err := resolver.server.productClient.UpdateProduct(ctx, in.ID, in.Name, in.Description, in.Price, int64(accountID))
	if err != nil {
		return nil, err
	}
	return toGeneratedProduct(product), nil
}

func (resolver *mutationResolver) DeleteProduct(ctx context.Context, id string) (*bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := resolver.server.productClient.DeleteProduct(ctx, id, int64(accountID)); err != nil {
		return nil, err
	}
	ok := true
	return &ok, nil
}

func (resolver *mutationResolver) CreateOrder(ctx context.Context, in generated.OrderInput) (*generated.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var products []*ordermodels.OrderedProduct
	for _, product := range in.Products {
		if product.Quantity <= 0 {
			return nil, ErrInvalidParameter
		}
		products = append(products, &ordermodels.OrderedProduct{
			ID:       product.ID,
			Quantity: uint32(product.Quantity),
		})
	}

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, errors.New("unauthorized")
	}

	postOrder, err := resolver.server.orderClient.PostOrder(ctx, uint64(accountID), products)
	if err != nil {
		return nil, err
	}
	return toGeneratedOrder(postOrder), nil
}

func (resolver *mutationResolver) CreateCustomerPortalSession(ctx context.Context, credentials *generated.CustomerPortalSessionInput) (*generated.RedirectResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url, err := resolver.server.paymentClient.CreateCustomerPortalSession(ctx, uint64(credentials.AccountID), credentials.Email, credentials.Name)
	if err != nil {
		return nil, err
	}
	return &generated.RedirectResponse{URL: url}, nil
}

func (resolver *mutationResolver) CreateCheckoutSession(ctx context.Context, details *generated.CheckoutInput) (*generated.RedirectResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var products []*paymentpb.CheckoutCartItem
	for _, product := range details.Products {
		products = append(products, &paymentpb.CheckoutCartItem{
			ProductId: product.ID,
			Quantity:  uint64(product.Quantity),
		})
	}
	url, err := resolver.server.paymentClient.CreateCheckoutSession(ctx, details.OrderID, details.AccountID, details.Email, details.Name, details.RedirectURL, products, nil)
	if err != nil {
		return nil, err
	}
	return &generated.RedirectResponse{URL: url}, nil
}

func (resolver *mutationResolver) UpsertProductStock(ctx context.Context, input generated.UpsertProductStockInput) (*generated.InventoryAvailability, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	product, err := resolver.server.productClient.GetProduct(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product.AccountID != accountID {
		return nil, errors.New("unauthorized")
	}

	_, availability, err := resolver.server.inventoryClient.UpsertStock(ctx, input.ProductID, int32(input.Quantity), int32(input.ReorderLevel))
	if err != nil {
		return nil, err
	}
	return toGeneratedAvailability(availability), nil
}

func (resolver *mutationResolver) AddCartItem(ctx context.Context, productID string, quantity int) (*generated.Cart, error) {
	return resolver.upsertCartItem(ctx, productID, quantity)
}

func (resolver *mutationResolver) UpdateCartItemQuantity(ctx context.Context, productID string, quantity int) (*generated.Cart, error) {
	return resolver.upsertCartItem(ctx, productID, quantity)
}

func (resolver *mutationResolver) RemoveCartItem(ctx context.Context, productID string) (*generated.Cart, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	cartSnapshot, err := resolver.server.cartClient.RemoveCartItem(ctx, uint64(accountID), productID)
	if err != nil {
		return nil, err
	}
	return buildCart(ctx, resolver.server, cartSnapshot)
}

func (resolver *mutationResolver) ClearCart(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return false, err
	}
	if err := resolver.server.cartClient.ClearCart(ctx, uint64(accountID)); err != nil {
		return false, err
	}
	return true, nil
}

func (resolver *mutationResolver) CheckoutCart(ctx context.Context, redirectURL string) (*generated.RedirectResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	account, err := resolver.server.accountClient.GetAccount(ctx, uint64(accountID))
	if err != nil {
		return nil, err
	}
	cartSnapshot, err := resolver.server.cartClient.PrepareCheckout(ctx, uint64(accountID))
	if err != nil {
		return nil, err
	}
	if len(cartSnapshot.Items) == 0 {
		return nil, errors.New("cart is empty")
	}

	var orderProducts []*ordermodels.OrderedProduct
	var paymentProducts []*paymentpb.CheckoutCartItem
	var reservationIDs []string
	for _, item := range cartSnapshot.Items {
		orderProducts = append(orderProducts, &ordermodels.OrderedProduct{ID: item.ProductID, Quantity: uint32(item.Quantity)})
		paymentProducts = append(paymentProducts, &paymentpb.CheckoutCartItem{ProductId: item.ProductID, Quantity: uint64(item.Quantity)})
		reservationIDs = append(reservationIDs, item.ReservationID)
	}

	createdOrder, err := resolver.server.orderClient.PostOrder(ctx, uint64(accountID), orderProducts)
	if err != nil {
		return nil, err
	}
	url, err := resolver.server.paymentClient.CreateCheckoutSession(ctx, int(createdOrder.ID), accountID, account.Email, account.Name, redirectURL, paymentProducts, reservationIDs)
	if err != nil {
		return nil, err
	}
	return &generated.RedirectResponse{URL: url}, nil
}

func (resolver *mutationResolver) MarkNotificationRead(ctx context.Context, id int) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return false, err
	}
	return resolver.server.notificationClient.MarkRead(ctx, uint64(accountID), uint64(id))
}

func (resolver *mutationResolver) MarkAllNotificationsRead(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return 0, err
	}
	count, err := resolver.server.notificationClient.MarkAllRead(ctx, uint64(accountID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (resolver *mutationResolver) upsertCartItem(ctx context.Context, productID string, quantity int) (*generated.Cart, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if quantity <= 0 {
		return nil, ErrInvalidParameter
	}
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	cartSnapshot, err := resolver.server.cartClient.UpsertCartItem(ctx, uint64(accountID), productID, int32(quantity))
	if err != nil {
		return nil, err
	}
	return buildCart(ctx, resolver.server, cartSnapshot)
}

func toGeneratedOrder(order *ordermodels.Order) *generated.Order {
	products := make([]*generated.OrderedProduct, 0, len(order.Products))
	for _, orderedProduct := range order.Products {
		products = append(products, &generated.OrderedProduct{
			ID:          orderedProduct.ID,
			Name:        orderedProduct.Name,
			Description: orderedProduct.Description,
			Price:       orderedProduct.Price,
			Quantity:    int(orderedProduct.Quantity),
		})
	}
	return &generated.Order{
		ID:         int(order.ID),
		CreatedAt:  order.CreatedAt,
		TotalPrice: order.TotalPrice,
		Products:   products,
	}
}

func toGeneratedAvailability(availability *inventorymodels.Availability) *generated.InventoryAvailability {
	return &generated.InventoryAvailability{
		ProductID:         availability.ProductID,
		TotalQuantity:     int(availability.TotalQuantity),
		ReservedQuantity:  int(availability.ReservedQuantity),
		AvailableQuantity: int(availability.AvailableQuantity),
		ReorderLevel:      int(availability.ReorderLevel),
		LowStock:          availability.HasActiveLowStock,
	}
}


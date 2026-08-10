package graph

import (
	"context"
	"errors"
	"log"
	"time"

	accountmodels "github.com/Tuananh165art/GoshopX/account/models"
	cartmodels "github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/Tuananh165art/GoshopX/graphql/generated"
	"github.com/Tuananh165art/GoshopX/graphql/utils"
	ordermodels "github.com/Tuananh165art/GoshopX/order/models"
	paymentmodels "github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/pkg/auth"
	productmodels "github.com/Tuananh165art/GoshopX/product/models"
)

type queryResolver struct {
	server *Server
}

func (resolver *queryResolver) Me(ctx context.Context) (*generated.Account, error) {
	accountID, err := auth.GetUserIdInt(ctx, false)
	if err != nil {
		return nil, nil
	}
	account, err := resolver.server.accountClient.GetAccount(ctx, uint64(accountID))
	if err != nil {
		return nil, err
	}
	return toGeneratedAccount(account), nil
}

func (resolver *queryResolver) CanReviewProduct(ctx context.Context, productID string) (bool, error) {
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return false, nil
	}
	return (&mutationResolver{server: resolver.server}).accountCanReviewProduct(ctx, uint64(accountID), productID)
}

func (resolver *queryResolver) Accounts(ctx context.Context, pagination *generated.PaginationInput, id *int) ([]*generated.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}

	if id != nil {
		res, err := resolver.server.accountClient.GetAccount(ctx, uint64(*id))
		if err != nil {
			return nil, err
		}
		return []*generated.Account{toGeneratedAccount(res)}, nil
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
		accounts = append(accounts, toGeneratedAccount(&account))
	}
	return accounts, nil
}

func (resolver *queryResolver) AdminAccounts(ctx context.Context, pagination *generated.PaginationInput) ([]*generated.Account, error) {
	if !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	return resolver.Accounts(ctx, pagination, nil)
}

func toGeneratedAccount(account *accountmodels.Account) *generated.Account {
	role := generated.AccountRoleCustomer
	if account.RoleID == accountmodels.RoleAdminID {
		role = generated.AccountRoleAdmin
	} else {
		switch account.Role {
		case "seller":
			role = generated.AccountRoleSeller
		case "support_admin":
			role = generated.AccountRoleSupportAdmin
		case "operations_admin":
			role = generated.AccountRoleOperationsAdmin
		case "platform_admin":
			role = generated.AccountRolePlatformAdmin
		}
	}
	status := generated.AccountStatusActive
	if account.Status == "suspended" {
		status = generated.AccountStatusSuspended
	}
	return &generated.Account{ID: int(account.ID), Name: account.Name, Email: account.Email, AvatarURL: account.AvatarURL, Phone: account.Phone, ShippingAddress: account.ShippingAddress, RoleID: account.RoleID, Role: role, Status: status}
}

func (resolver *queryResolver) AdminProducts(ctx context.Context, pagination *generated.PaginationInput) ([]*generated.Product, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	skip, take := utils.Bounds(pagination)
	products, err := resolver.server.productClient.GetAllProducts(ctx, skip, take)
	if err != nil {
		return nil, err
	}
	result := make([]*generated.Product, 0, len(products))
	for i := range products {
		product := products[i]
		result = append(result, resolver.toGeneratedProductWithAvailability(ctx, &product))
	}
	return result, nil
}

func (resolver *queryResolver) Product(ctx context.Context, pagination *generated.PaginationInput, query, category, id *string, viewedProductsIds []*string, byAccountID *bool) ([]*generated.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if id != nil {
		res, err := resolver.server.productClient.GetProduct(ctx, *id)
		if err != nil {
			return nil, err
		}
		return []*generated.Product{resolver.toGeneratedProductWithAvailability(ctx, res)}, nil
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
	var productList []productmodels.Product
	var err error
	if category != nil && *category != "" && q == "" {
		productList, err = resolver.server.productClient.GetProductsByCategory(ctx, *category, skip, take)
	} else {
		productList, err = resolver.server.productClient.GetProducts(ctx, skip, take, nil, q)
	}
	if err != nil {
		return nil, err
	}

	var products []*generated.Product
	for _, product := range productList {
		productCopy := product
		products = append(products, resolver.toGeneratedProductWithAvailability(ctx, &productCopy))
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

func (resolver *queryResolver) MyOrders(ctx context.Context) ([]*generated.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, false)
	if err != nil {
		return nil, err
	}
	orders, err := resolver.server.orderClient.GetOrdersForAccount(ctx, uint64(accountID))
	if err != nil {
		return nil, err
	}
	result := make([]*generated.Order, 0, len(orders))
	for index := range orders {
		result = append(result, toGeneratedOrder(&orders[index]))
	}
	return result, nil
}

func (resolver *queryResolver) MyPaymentTransactions(ctx context.Context, orderID int) ([]*generated.PaymentTransaction, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	accountID, err := auth.GetUserIdInt(ctx, false)
	if err != nil {
		return nil, err
	}
	items, err := resolver.server.paymentClient.ListTransactions(ctx, "", uint64(orderID), 0, 50)
	if err != nil {
		return nil, err
	}
	result := make([]*generated.PaymentTransaction, 0, len(items))
	for _, item := range items {
		if item.UserId == uint64(accountID) {
			result = append(result, toGeneratedTransaction(item))
		}
	}
	return result, nil
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

func (resolver *queryResolver) AdminDashboard(ctx context.Context, from time.Time, to time.Time, topProductsLimit *int) (*generated.AdminDashboard, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	limit := 10
	if topProductsLimit != nil {
		limit = *topProductsLimit
	}
	dashboard, err := resolver.server.adminClient.Dashboard(ctx, from, to, limit)
	if err != nil {
		return nil, err
	}
	result := &generated.AdminDashboard{Gmv: float64(dashboard.GMV), Revenue: float64(dashboard.Revenue), OrderCount: int(dashboard.OrderCount), AverageOrderValue: dashboard.AverageOrderValue, PaymentAttempts: int(dashboard.PaymentAttempts), SuccessfulPayments: int(dashboard.SuccessfulPayments), PaymentSuccessRate: dashboard.PaymentSuccessRate}
	for _, metric := range dashboard.TopProducts {
		result.TopProducts = append(result.TopProducts, &generated.AdminProductMetric{ProductID: metric.ProductID, Quantity: int(metric.Quantity), Gmv: float64(metric.GMV)})
	}
	return result, nil
}

func (resolver *queryResolver) AdminAuditEvents(ctx context.Context, pagination *generated.PaginationInput, actorAccountID *int, action *string, from *time.Time, to *time.Time) ([]*generated.AdminAuditEvent, error) {
	if !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	skip, take := uint64(0), uint64(20)
	if pagination != nil {
		skip, take = utils.Bounds(pagination)
	}
	var actorID uint64
	if actorAccountID != nil {
		actorID = uint64(*actorAccountID)
	}
	var actionValue string
	if action != nil {
		actionValue = *action
	}
	fromValue, toValue := time.Unix(0, 0).UTC(), time.Now().UTC().Add(24*time.Hour)
	if from != nil {
		fromValue = *from
	}
	if to != nil {
		toValue = *to
	}
	events, err := resolver.server.adminClient.ListAudit(ctx, actorID, actionValue, fromValue, toValue, int(skip), int(take))
	if err != nil {
		return nil, err
	}
	result := make([]*generated.AdminAuditEvent, 0, len(events))
	for _, event := range events {
		result = append(result, &generated.AdminAuditEvent{EventID: event.EventID, ActorAccountID: int(event.ActorAccountID), TargetAccountID: int(event.TargetAccountID), Action: event.Action, Outcome: event.Outcome, RequestID: event.RequestID, OccurredAt: event.OccurredAt})
	}
	return result, nil
}

func (resolver *queryResolver) Categories(ctx context.Context, activeOnly bool) ([]*generated.Category, error) {
	if !activeOnly && !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	items, err := resolver.server.productClient.ListCategories(ctx, activeOnly)
	if err != nil {
		return nil, err
	}
	result := make([]*generated.Category, 0, len(items))
	for _, item := range items {
		result = append(result, &generated.Category{ID: item.ID, Name: item.Name, Slug: item.Slug, Description: item.Description, IsActive: item.IsActive})
	}
	return result, nil
}

func toGeneratedAdminOrder(order *ordermodels.Order) *generated.AdminOrder {
	return &generated.AdminOrder{ID: int(order.ID), AccountID: int(order.AccountID), TotalPrice: order.TotalPrice, Status: order.Status, PaymentStatus: order.PaymentStatus, CreatedAt: order.CreatedAt, Products: toGeneratedOrder(order).Products}
}
func toGeneratedTransaction(item *paymentmodels.Transaction) *generated.PaymentTransaction {
	return &generated.PaymentTransaction{OrderID: int(item.OrderId), UserID: int(item.UserId), PaymentID: item.PaymentId, TotalPrice: int(item.TotalPrice), SettledPrice: int(item.SettledPrice), Currency: item.Currency, Status: item.Status}
}
func (resolver *queryResolver) AdminOrders(ctx context.Context, filter *generated.AdminOrderFilter) ([]*generated.AdminOrder, error) {
	if !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	status, paymentStatus := "", ""
	var accountID, skip, take uint64
	take = 20
	if filter != nil {
		if filter.Status != nil {
			status = *filter.Status
		}
		if filter.PaymentStatus != nil {
			paymentStatus = *filter.PaymentStatus
		}
		if filter.AccountID != nil {
			accountID = uint64(*filter.AccountID)
		}
		if filter.Pagination != nil {
			skip, take = utils.Bounds(filter.Pagination)
		}
	}
	orders, err := resolver.server.orderClient.ListOrders(ctx, status, paymentStatus, accountID, skip, take)
	if err != nil {
		return nil, err
	}
	result := make([]*generated.AdminOrder, 0, len(orders))
	for _, item := range orders {
		result = append(result, toGeneratedAdminOrder(item))
	}
	return result, nil
}
func (resolver *queryResolver) AdminOrder(ctx context.Context, id int) (*generated.AdminOrder, error) {
	if !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	item, err := resolver.server.orderClient.GetOrder(ctx, uint64(id))
	if err != nil {
		return nil, err
	}
	return toGeneratedAdminOrder(item), nil
}
func (resolver *queryResolver) AdminTransactions(ctx context.Context, status *string, orderID *int, pagination *generated.PaginationInput) ([]*generated.PaymentTransaction, error) {
	if !auth.HasAnyRole(ctx, "platform_admin") {
		return nil, errors.New("forbidden")
	}
	value := ""
	if status != nil {
		value = *status
	}
	var id uint64
	if orderID != nil {
		id = uint64(*orderID)
	}
	skip, take := uint64(0), uint64(20)
	if pagination != nil {
		skip, take = utils.Bounds(pagination)
	}
	items, err := resolver.server.paymentClient.ListTransactions(ctx, value, id, skip, take)
	if err != nil {
		return nil, err
	}
	result := make([]*generated.PaymentTransaction, 0, len(items))
	for _, item := range items {
		result = append(result, toGeneratedTransaction(item))
	}
	return result, nil
}
func (resolver *queryResolver) AdminReconcileTransaction(ctx context.Context, paymentID string) (*generated.PaymentTransaction, error) {
	if !auth.HasAnyRole(ctx, "platform_admin") {
		return nil, errors.New("forbidden")
	}
	item, err := resolver.server.paymentClient.ReconcileTransaction(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	return toGeneratedTransaction(item), nil
}
func (resolver *queryResolver) AdminLowStock(ctx context.Context, limit int) ([]*generated.InventoryAvailability, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	stocks, err := resolver.server.inventoryClient.ListLowStock(ctx, uint64(limit))
	if err != nil {
		return nil, err
	}
	result := make([]*generated.InventoryAvailability, 0, len(stocks))
	for _, stock := range stocks {
		availability, err := resolver.server.inventoryClient.GetAvailability(ctx, stock.ProductID)
		if err != nil {
			return nil, err
		}
		result = append(result, toGeneratedAvailability(availability))
	}
	return result, nil
}
func (resolver *queryResolver) AdminReservations(ctx context.Context, status *string, pagination *generated.PaginationInput) ([]*generated.StockReservation, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	value := ""
	if status != nil {
		value = *status
	}
	skip, take := uint64(0), uint64(20)
	if pagination != nil {
		skip, take = utils.Bounds(pagination)
	}
	items, err := resolver.server.inventoryClient.ListReservations(ctx, value, skip, take)
	if err != nil {
		return nil, err
	}
	result := make([]*generated.StockReservation, 0, len(items))
	for _, item := range items {
		result = append(result, &generated.StockReservation{ReservationID: item.ReservationID, AccountID: int(item.AccountID), ProductID: item.ProductID, Quantity: int(item.Quantity), Status: string(item.Status), Source: item.Source, ExpiresAt: item.ExpiresAt})
	}
	return result, nil
}

func (resolver *queryResolver) toGeneratedProductWithAvailability(ctx context.Context, product *productmodels.Product) *generated.Product {
	result := toGeneratedProduct(product)
	availability, err := resolver.server.inventoryClient.GetAvailability(ctx, product.ID)
	if err == nil && availability != nil {
		result.Stock = int(availability.AvailableQuantity)
	}
	return result
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
		generatedProduct := toGeneratedProduct(&productCopy)
		if availability, availabilityErr := server.inventoryClient.GetAvailability(ctx, product.ID); availabilityErr == nil && availability != nil {
			generatedProduct.Stock = int(availability.AvailableQuantity)
		}
		productByID[product.ID] = generatedProduct
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
	media := make([]*generated.ProductMedia, 0, len(product.Media))
	for _, item := range product.Media {
		media = append(media, &generated.ProductMedia{ID: item.ID, URL: item.URL, AltText: item.AltText, SortOrder: item.SortOrder})
	}
	reviews := make([]*generated.ProductReview, 0, len(product.Reviews))
	for _, item := range product.Reviews {
		reviews = append(reviews, &generated.ProductReview{Rating: item.Rating, Comment: item.Comment, Date: item.Date, ReviewerName: item.ReviewerName, ReviewerEmail: item.ReviewerEmail})
	}
	return &generated.Product{
		ID: product.ID, Name: product.Name, Description: product.Description, Price: product.Price,
		AccountID: product.AccountID, CategoryID: product.CategoryID, PublishStatus: product.PublishStatus,
		ModerationStatus: product.ModerationStatus, ModerationReason: product.ModerationReason, Media: media,
		DiscountPercentage: product.DiscountPercentage, Rating: product.Rating, Stock: product.Stock,
		Tags: product.Tags, Brand: product.Brand, Sku: product.SKU, Weight: product.Weight,
		Dimensions:          &generated.ProductDimensions{Width: product.Dimensions.Width, Height: product.Dimensions.Height, Depth: product.Dimensions.Depth},
		WarrantyInformation: product.WarrantyInformation, ShippingInformation: product.ShippingInformation,
		AvailabilityStatus: product.AvailabilityStatus, Reviews: reviews, ReturnPolicy: product.ReturnPolicy,
		MinimumOrderQuantity: product.MinimumOrderQuantity,
		Meta:                 &generated.ProductMeta{CreatedAt: product.Meta.CreatedAt, UpdatedAt: product.Meta.UpdatedAt, Barcode: product.Meta.Barcode, QRCode: product.Meta.QRCode},
		Thumbnail:            product.Thumbnail, Images: product.Images,
	}
}

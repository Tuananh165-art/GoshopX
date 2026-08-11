package graph

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/99designs/gqlgen/graphql"
	cartmodels "github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/Tuananh165art/GoshopX/graphql/generated"
	inventorymodels "github.com/Tuananh165art/GoshopX/inventory/models"
	ordermodels "github.com/Tuananh165art/GoshopX/order/models"
	paymentpb "github.com/Tuananh165art/GoshopX/payment/proto/pb"
	"github.com/Tuananh165art/GoshopX/pkg/auth"
	"github.com/Tuananh165art/GoshopX/pkg/contextkeys"
	"github.com/Tuananh165art/GoshopX/pkg/middleware"
	productmodels "github.com/Tuananh165art/GoshopX/product/models"
)

var ErrInvalidParameter = errors.New("invalid parameter")

const codPaymentStatus = "paid"

func setSessionCookie(ctx *gin.Context, token string, maxAge int) {
	// An empty Domain makes this a host-only cookie. Hard-coding localhost
	// prevents browsers from accepting it when GraphQL is served by a domain.
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("token", token, maxAge, "/", "", requestUsesHTTPS(ctx), true)
}

func requestUsesHTTPS(ctx *gin.Context) bool {
	if ctx.Request != nil && ctx.Request.TLS != nil {
		return true
	}
	forwardedProto := strings.TrimSpace(strings.Split(ctx.GetHeader("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(forwardedProto, "https")
}

type mutationResolver struct {
	server *Server
}

func (resolver *mutationResolver) UpdateMyProfile(ctx context.Context, in generated.ProfileInput) (*generated.Account, error) {
	accountID, err := auth.GetUserIdInt(ctx, false)
	if err != nil {
		return nil, err
	}
	account, err := resolver.server.accountClient.UpdateProfile(ctx, uint64(accountID), in.Name, in.Email, in.AvatarURL, in.Phone, in.ShippingAddress)
	if err != nil {
		return nil, err
	}
	return toGeneratedAccount(account), nil
}

func (resolver *mutationResolver) RequestPasswordReset(ctx context.Context, email string) (bool, error) {
	if err := resolver.server.accountClient.RequestPasswordReset(ctx, email); err != nil {
		return false, err
	}
	return true, nil
}

func (resolver *mutationResolver) ResetPassword(ctx context.Context, email, otp, newPassword string) (bool, error) {
	if err := resolver.server.accountClient.ResetPassword(ctx, email, otp, newPassword); err != nil {
		return false, err
	}
	return true, nil
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
	setSessionCookie(ginContext, token, 3600)
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
	setSessionCookie(ginContext, token, 3600)
	return &generated.AuthResponse{Token: token}, nil
}

func (resolver *mutationResolver) LoginWithGoogle(ctx context.Context, in generated.GoogleLoginInput) (*generated.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	token, err := resolver.server.accountClient.LoginWithGoogle(ctx, in.Credential)
	if err != nil {
		return nil, err
	}
	ginContext, ok := middleware.GinContextFromContext(ctx)
	if !ok {
		return nil, errors.New("could not retrieve gin context")
	}
	setSessionCookie(ginContext, token, 3600)
	return &generated.AuthResponse{Token: token}, nil
}

func (resolver *mutationResolver) Logout(ctx context.Context) (bool, error) {
	ginContext, ok := middleware.GinContextFromContext(ctx)
	if !ok {
		return false, errors.New("could not retrieve gin context")
	}
	setSessionCookie(ginContext, "", -1)
	return true, nil
}

func (resolver *mutationResolver) RecommendChat(ctx context.Context, in generated.RecommendationChatInput) (*generated.RecommendationChatResponse, error) {
	// A chat completion can consume the configured 30s provider timeout and
	// retries; this boundary must not cancel gRPC before recommender finishes.
	ctx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()

	userID := ""
	if accountID, err := auth.GetUserIdInt(ctx, false); err == nil {
		userID = fmt.Sprintf("%d", accountID)
	}
	imageURL := ""
	if in.ImageURL != nil {
		imageURL = *in.ImageURL
	}
	if in.Image != nil {
		encoded, err := encodeChatImage(*in.Image)
		if err != nil {
			return nil, err
		}
		imageURL = encoded
	}
	sessionID := ""
	if in.SessionID != nil {
		sessionID = *in.SessionID
	}
	limit := uint32(5)
	if in.Limit != nil && *in.Limit > 0 {
		limit = uint32(*in.Limit)
	}
	response, err := resolver.server.recommenderClient.ChatRecommend(ctx, in.Text, imageURL, userID, sessionID, in.ViewedProductIds, limit, uuid.NewString())
	if err != nil {
		return nil, err
	}
	result := &generated.RecommendationChatResponse{Answer: response.Answer, NextSuggestions: response.NextSuggestions, Warnings: response.Warnings, RequestID: response.RequestId}
	for _, product := range response.Products {
		result.Products = append(result.Products, &generated.RecommendationChatProduct{
			ID: product.Id, Name: product.Name, Description: product.Description, Price: product.Price,
			Currency: product.Currency, Thumbnail: product.Thumbnail, Images: product.Images,
			Rating: product.Rating, Stock: int(product.Stock), RetrievalScore: product.RetrievalScore,
			MatchedReasons: product.MatchedReasons,
		})
	}
	return result, nil
}

const maxChatImageBytes = 3 << 20

func encodeChatImage(upload graphql.Upload) (string, error) {
	if upload.File == nil || upload.Size <= 0 || upload.Size > maxChatImageBytes {
		return "", errors.New("chat image must be between 1 byte and 3 MiB")
	}
	contentType := strings.ToLower(strings.TrimSpace(upload.ContentType))
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
	if !allowed[contentType] {
		return "", errors.New("chat image must be jpeg, png, or webp")
	}
	data, err := io.ReadAll(io.LimitReader(upload.File, maxChatImageBytes+1))
	if err != nil {
		return "", errors.New("could not read chat image")
	}
	if len(data) == 0 || len(data) > maxChatImageBytes {
		return "", errors.New("chat image exceeds the 3 MiB limit")
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
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
	clientIP, _ := ctx.Value(contextkeys.ClientIPKey).(string)
	url, err := resolver.server.paymentClient.CreateCheckoutSession(ctx, details.OrderID, details.AccountID, details.Email, details.Name, details.RedirectURL, clientIP, products, nil)
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
	return resolver.addCartItem(ctx, productID, quantity, true)
}

func (resolver *mutationResolver) UpdateCartItemQuantity(ctx context.Context, productID string, quantity int) (*generated.Cart, error) {
	return resolver.addCartItem(ctx, productID, quantity, false)
}

func (resolver *mutationResolver) AddProductReview(ctx context.Context, productID string, rating int, comment string) (*generated.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, errors.New("unauthorized")
	}
	eligible, err := resolver.accountCanReviewProduct(ctx, uint64(accountID), productID)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, errors.New("only customers who completed payment can review this product")
	}
	account, err := resolver.server.accountClient.GetAccount(ctx, uint64(accountID))
	if err != nil {
		return nil, err
	}
	product, err := resolver.server.productClient.AddReview(ctx, productID, rating, comment, account.Name, account.Email)
	if err != nil {
		return nil, err
	}
	return toGeneratedProduct(product), nil
}

func (resolver *mutationResolver) accountCanReviewProduct(ctx context.Context, accountID uint64, productID string) (bool, error) {
	orders, err := resolver.server.orderClient.GetOrdersForAccount(ctx, accountID)
	if err != nil {
		return false, err
	}
	for _, order := range orders {
		status := strings.ToLower(strings.TrimSpace(order.PaymentStatus))
		if status != "paid" && status != "succeeded" && status != "success" && status != "completed" {
			continue
		}
		for _, item := range order.Products {
			if item.ID == productID {
				return true, nil
			}
		}
	}
	return false, nil
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
	clientIP, _ := ctx.Value(contextkeys.ClientIPKey).(string)
	url, err := resolver.server.paymentClient.CreateCheckoutSession(ctx, int(createdOrder.ID), accountID, account.Email, account.Name, redirectURL, clientIP, paymentProducts, reservationIDs)
	if err != nil {
		return nil, err
	}
	return &generated.RedirectResponse{URL: url}, nil
}

func (resolver *mutationResolver) CheckoutCartCod(ctx context.Context) (*generated.CODCheckoutResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, errors.New("unauthorized")
	}
	cartSnapshot, err := resolver.server.cartClient.PrepareCheckout(ctx, uint64(accountID))
	if err != nil {
		return nil, err
	}
	if len(cartSnapshot.Items) == 0 {
		return nil, errors.New("cart is empty")
	}

	orderProducts := make([]*ordermodels.OrderedProduct, 0, len(cartSnapshot.Items))
	for _, item := range cartSnapshot.Items {
		orderProducts = append(orderProducts, &ordermodels.OrderedProduct{ID: item.ProductID, Quantity: uint32(item.Quantity)})
	}
	createdOrder, err := resolver.server.orderClient.PostOrder(ctx, uint64(accountID), orderProducts)
	if err != nil {
		return nil, err
	}
	for _, item := range cartSnapshot.Items {
		if item.ReservationID == "" {
			continue
		}
		if _, _, err := resolver.server.inventoryClient.CommitReservation(ctx, item.ReservationID); err != nil {
			return nil, err
		}
	}
	if err := resolver.server.orderClient.UpdateOrderStatus(ctx, uint64(createdOrder.ID), codPaymentStatus); err != nil {
		return nil, err
	}
	if err := resolver.server.cartClient.CompleteCheckout(ctx, uint64(accountID)); err != nil {
		return nil, err
	}
	return &generated.CODCheckoutResponse{OrderID: int(createdOrder.ID), Status: codPaymentStatus}, nil
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

func (resolver *mutationResolver) SuspendAccount(ctx context.Context, id int) (bool, error) {
	actorID, err := auth.GetUserIdInt(ctx, true)
	if err != nil || !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return false, errors.New("forbidden")
	}
	_, err = resolver.server.accountClient.SetAccountStatus(ctx, uint64(actorID), uint64(id), auth.GetRole(ctx), "suspended", fmt.Sprintf("graphql-%d", time.Now().UnixNano()))
	return err == nil, err
}

func (resolver *mutationResolver) ReactivateAccount(ctx context.Context, id int) (bool, error) {
	actorID, err := auth.GetUserIdInt(ctx, true)
	if err != nil || !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return false, errors.New("forbidden")
	}
	_, err = resolver.server.accountClient.SetAccountStatus(ctx, uint64(actorID), uint64(id), auth.GetRole(ctx), "active", fmt.Sprintf("graphql-%d", time.Now().UnixNano()))
	return err == nil, err
}

func (resolver *mutationResolver) SetAccountRole(ctx context.Context, id int, role generated.AccountRole) (*generated.Account, error) {
	actorID, err := auth.GetUserIdInt(ctx, true)
	if err != nil || !auth.HasAnyRole(ctx, "platform_admin") {
		return nil, errors.New("forbidden")
	}
	account, err := resolver.server.accountClient.SetAccountRole(ctx, uint64(actorID), uint64(id), auth.GetRole(ctx), strings.ToLower(string(role)), fmt.Sprintf("graphql-%d", time.Now().UnixNano()))
	if err != nil {
		return nil, err
	}
	return toGeneratedAccount(account), nil
}

func (resolver *mutationResolver) AdminCreateCategory(ctx context.Context, input generated.CreateCategoryInput) (*generated.Category, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	category, err := resolver.server.productClient.CreateCategory(ctx, input.Name, input.Slug, input.Description, auth.GetRole(ctx), input.IsActive)
	if err != nil {
		return nil, err
	}
	return &generated.Category{ID: category.ID, Name: category.Name, Slug: category.Slug, Description: category.Description, IsActive: category.IsActive}, nil
}

func (resolver *mutationResolver) AdminModerateProduct(ctx context.Context, input generated.ModerateProductInput) (*generated.Product, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	product, err := resolver.server.productClient.ModerateProduct(ctx, input.ProductID, input.PublishStatus, input.ModerationStatus, input.ModerationReason, auth.GetRole(ctx))
	if err != nil {
		return nil, err
	}
	return toGeneratedProduct(product), nil
}

func (resolver *mutationResolver) AdminAddProductMedia(ctx context.Context, input generated.AddProductMediaInput) (*generated.Product, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	product, err := resolver.server.productClient.AddProductMedia(ctx, input.ProductID, input.URL, input.AltText, auth.GetRole(ctx), int32(input.SortOrder))
	if err != nil {
		return nil, err
	}
	return toGeneratedProduct(product), nil
}

func (resolver *mutationResolver) AdminUploadProductMedia(ctx context.Context, productID string, file graphql.Upload, altText string, sortOrder int) (*generated.Product, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	if resolver.server.mediaStore == nil {
		return nil, errors.New("minio media storage is not configured")
	}
	url, checksum, err := resolver.server.mediaStore.uploadProductMedia(ctx, productID, file)
	if err != nil {
		return nil, err
	}
	product, err := resolver.server.productClient.AddProductMedia(ctx, productID, url, altText, auth.GetRole(ctx), int32(sortOrder))
	if err != nil {
		return nil, err
	}
	_ = checksum // The object key is unique; checksum is retained in MinIO object metadata in the next migration.
	return toGeneratedProduct(product), nil
}

func (resolver *mutationResolver) AdminCreateProduct(ctx context.Context, input generated.AdminProductInput) (*generated.Product, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	created, err := resolver.server.productClient.PostProduct(ctx, input.Name, input.Description, input.Price, int64(accountID))
	if err != nil {
		return nil, err
	}
	created.CategoryID, created.Brand, created.SKU = input.CategoryID, input.Brand, input.Sku
	created.Thumbnail, created.Images, created.Tags = input.Thumbnail, input.Images, input.Tags
	if input.PublishStatus != nil {
		created.PublishStatus = *input.PublishStatus
	}
	if input.ModerationStatus != nil {
		created.ModerationStatus = *input.ModerationStatus
	}
	if input.ModerationReason != nil {
		created.ModerationReason = *input.ModerationReason
	}
	updated, err := resolver.server.productClient.AdminUpdateProduct(ctx, created, auth.GetRole(ctx))
	if err != nil {
		return nil, err
	}
	return toGeneratedProduct(updated), nil
}

func (resolver *mutationResolver) AdminUpdateProduct(ctx context.Context, input generated.AdminProductInput) (*generated.Product, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") || input.ID == nil || strings.TrimSpace(*input.ID) == "" {
		return nil, errors.New("forbidden or product id is missing")
	}
	product := &productmodels.Product{ID: *input.ID, Name: input.Name, Description: input.Description, Price: input.Price, CategoryID: input.CategoryID, Brand: input.Brand, SKU: input.Sku, Thumbnail: input.Thumbnail, Images: input.Images, Tags: input.Tags}
	if input.PublishStatus != nil {
		product.PublishStatus = *input.PublishStatus
	}
	if input.ModerationStatus != nil {
		product.ModerationStatus = *input.ModerationStatus
	}
	if input.ModerationReason != nil {
		product.ModerationReason = *input.ModerationReason
	}
	updated, err := resolver.server.productClient.AdminUpdateProduct(ctx, product, auth.GetRole(ctx))
	if err != nil {
		return nil, err
	}
	return toGeneratedProduct(updated), nil
}

func (resolver *mutationResolver) AdminDeleteProduct(ctx context.Context, id string) (bool, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return false, errors.New("forbidden")
	}
	if err := resolver.server.productClient.AdminDeleteProduct(ctx, id, auth.GetRole(ctx)); err != nil {
		return false, err
	}
	return true, nil
}

func (resolver *mutationResolver) AdminCancelOrder(ctx context.Context, orderID int, reason string) (*generated.AdminOrder, error) {
	if !auth.HasAnyRole(ctx, "support_admin", "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	order, err := resolver.server.orderClient.CancelOrder(ctx, uint64(orderID), reason, auth.GetRole(ctx))
	if err != nil {
		return nil, err
	}
	return toGeneratedAdminOrder(order), nil
}
func (resolver *mutationResolver) AdminRequestRefund(ctx context.Context, input generated.RefundInput) (*generated.RefundResult, error) {
	if !auth.HasAnyRole(ctx, "platform_admin") {
		return nil, errors.New("forbidden")
	}
	result, err := resolver.server.paymentClient.RequestRefund(ctx, input.PaymentID, input.Reason, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return &generated.RefundResult{ProviderRefundID: result.ProviderRefundId, Status: result.Status, Amount: int(result.Amount)}, nil
}
func (resolver *mutationResolver) AdminAdjustInventory(ctx context.Context, input generated.InventoryAdjustmentInput) (*generated.InventoryAvailability, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	_, availability, err := resolver.server.inventoryClient.AdjustStock(ctx, input.ProductID, int32(input.Delta), int32(input.ReorderLevel), input.Reason)
	if err != nil {
		return nil, err
	}
	return toGeneratedAvailability(availability), nil
}

func (resolver *mutationResolver) addCartItem(ctx context.Context, productID string, quantity int, increment bool) (*generated.Cart, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if quantity <= 0 {
		return nil, ErrInvalidParameter
	}
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	var cartSnapshot *cartmodels.Cart
	if increment {
		cartSnapshot, err = resolver.server.cartClient.AddCartItem(ctx, uint64(accountID), productID, int32(quantity))
	} else {
		cartSnapshot, err = resolver.server.cartClient.UpsertCartItem(ctx, uint64(accountID), productID, int32(quantity))
	}
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

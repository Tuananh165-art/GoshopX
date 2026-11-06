package graph

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/99designs/gqlgen/graphql"
	cartmodels "github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/Tuananh165art/GoshopX/graphql/config"
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

const codCheckoutStatus = "pending"

func parseCheckoutCartVersion(version *string) (uint64, error) {
	if version == nil || *version == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(*version, 10, 64)
	if err != nil || parsed == 0 {
		return 0, ErrInvalidParameter
	}
	return parsed, nil
}

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
	ginContext, hasGinContext := middleware.GinContextFromContext(ctx)
	if userID == "" && hasGinContext {
		userID = guestChatOwnerID(ginContext, config.SecretKey)
	}
	imageURL := ""
	if in.ImageURL != nil {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(*in.ImageURL)), "gs://") {
			return nil, errors.New("temporary image references are server-managed")
		}
		imageURL = *in.ImageURL
	}
	if in.Image != nil {
		encoded, err := encodeChatImage(*in.Image)
		if err != nil {
			return nil, err
		}
		if resolver.server.mediaStore != nil && resolver.server.mediaStore.gcs != nil {
			imageURL, err = resolver.server.mediaStore.storeChatImage(ctx, encoded)
			if err != nil {
				return nil, err
			}
		} else {
			imageURL = encoded
		}
	}
	sessionID := ""
	if in.SessionID != nil {
		sessionID = *in.SessionID
	}
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	limit := uint32(5)
	if in.Limit != nil && *in.Limit > 0 {
		limit = uint32(*in.Limit)
	}
	response, err := resolver.server.recommenderClient.ChatRecommend(ctx, in.Text, imageURL, userID, sessionID, in.ViewedProductIds, limit, uuid.NewString())
	if err != nil {
		return nil, err
	}
	result := &generated.RecommendationChatResponse{Answer: response.Answer, NextSuggestions: response.NextSuggestions, Warnings: response.Warnings, RequestID: response.RequestId, SessionID: response.SessionId}
	for _, source := range response.Sources {
		result.Sources = append(result.Sources, &generated.RecommendationChatSource{Title: source.Title, URL: source.Url, PublishedAt: source.PublishedAt})
	}
	for _, product := range response.Products {
		result.Products = append(result.Products, &generated.RecommendationChatProduct{
			ID: product.Id, Name: product.Name, Description: product.Description, Price: product.Price,
			Currency: product.Currency, Thumbnail: product.Thumbnail, Images: product.Images,
			Rating: product.Rating, Stock: int(product.Stock), RetrievalScore: product.RetrievalScore,
			MatchedReasons: product.MatchedReasons,
			Brand:          product.Brand, Sku: product.Sku, WarrantyInformation: product.WarrantyInformation,
			ShippingInformation: product.ShippingInformation, ReturnPolicy: product.ReturnPolicy,
		})
	}
	return result, nil
}

func (resolver *mutationResolver) DeleteChatSession(ctx context.Context, sessionID string) (bool, error) {
	if strings.TrimSpace(sessionID) == "" || len(sessionID) > 128 {
		return false, ErrInvalidParameter
	}
	userID := ""
	if accountID, err := auth.GetUserIdInt(ctx, false); err == nil {
		userID = fmt.Sprintf("%d", accountID)
	}
	if userID == "" {
		ginContext, ok := middleware.GinContextFromContext(ctx)
		if !ok {
			return false, errors.New("chat owner is unavailable")
		}
		userID = guestChatOwnerID(ginContext, config.SecretKey)
	}
	if userID == "" {
		return false, errors.New("chat guest signing is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return resolver.server.recommenderClient.DeleteChatHistory(ctx, userID, sessionID)
}

func guestChatOwnerID(ctx *gin.Context, secret string) string {
	const cookieName = "goshopx_chat_guest"
	if cookie, err := ctx.Request.Cookie(cookieName); err == nil {
		if id, ok := verifyGuestChatCookie(cookie.Value, secret); ok {
			return "guest:" + id
		}
	}
	id := uuid.NewString()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id))
	value := id + "." + hex.EncodeToString(mac.Sum(nil))
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(cookieName, value, 7*24*60*60, "/", "", requestUsesHTTPS(ctx), true)
	if secret == "" {
		return ""
	}
	return "guest:" + id
}

func verifyGuestChatCookie(value, secret string) (string, bool) {
	if secret == "" {
		return "", false
	}
	id, signature, ok := strings.Cut(value, ".")
	if !ok || id == "" {
		return "", false
	}
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id))
	return id, hmac.Equal(provided, mac.Sum(nil))
}

const maxChatImageBytes = 5 << 20

func encodeChatImage(upload graphql.Upload) (string, error) {
	if upload.File == nil || upload.Size <= 0 || upload.Size > maxChatImageBytes {
		return "", errors.New("chat image must be between 1 byte and 5 MiB")
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
		return "", errors.New("chat image exceeds the 5 MiB limit")
	}
	extension := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[contentType]
	if _, detectedType, err := validateProductMedia(bytes.NewReader(data), extension); err != nil || detectedType != contentType {
		return "", errors.New("chat image bytes do not match the declared image type")
	}
	data, err = normalizeChatImage(contentType, data)
	if err != nil {
		return "", err
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

	postOrder, err := resolver.server.orderClient.PostOrder(ctx, uint64(accountID), products, false)
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

func (resolver *mutationResolver) CreateCartProposal(ctx context.Context, expectedCartVersion, idempotencyKey string, items []*generated.CartProposalItemInput) (*generated.CartProposal, error) {
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	version, err := strconv.ParseUint(expectedCartVersion, 10, 64)
	if err != nil {
		return nil, ErrInvalidParameter
	}
	proposalItems := make([]cartmodels.ProposalItem, 0, len(items))
	for _, item := range items {
		if item == nil || item.Quantity <= 0 || item.Quantity > 100 {
			return nil, ErrInvalidParameter
		}
		proposalItems = append(proposalItems, cartmodels.ProposalItem{ProductID: item.ProductID, Quantity: int32(item.Quantity)})
	}
	proposal, err := resolver.server.cartClient.CreateCartProposal(ctx, uint64(accountID), version, idempotencyKey, proposalItems)
	if err != nil {
		return nil, err
	}
	return toGeneratedCartProposal(proposal), nil
}

func (resolver *mutationResolver) ConfirmCartProposal(ctx context.Context, proposalID string) (*generated.Cart, error) {
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	cart, err := resolver.server.cartClient.ConfirmCartProposal(ctx, uint64(accountID), proposalID)
	if err != nil {
		return nil, err
	}
	return buildCart(ctx, resolver.server, cart)
}

func toGeneratedCartProposal(proposal *cartmodels.CartProposal) *generated.CartProposal {
	items := make([]*generated.CartProposalItem, 0, len(proposal.Items))
	for _, item := range proposal.Items {
		items = append(items, &generated.CartProposalItem{ProductID: item.ProductID, Quantity: int(item.Quantity)})
	}
	return &generated.CartProposal{ID: proposal.ID, BaseVersion: strconv.FormatUint(proposal.BaseVersion, 10), ExpiresAt: proposal.ExpiresAt, Items: items}
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

func (resolver *mutationResolver) CheckoutCart(ctx context.Context, redirectURL, quoteID, cartVersion string) (*generated.RedirectResponse, error) {
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
	expectedVersion, err := parseCheckoutCartVersion(&cartVersion)
	if err != nil {
		return nil, err
	}
	cartSnapshot, err := resolver.server.cartClient.PrepareCheckoutAtVersion(ctx, uint64(accountID), expectedVersion)
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

	createdOrder, err := resolver.server.orderClient.PostOrderWithQuote(ctx, uint64(accountID), orderProducts, false, quoteID, cartSnapshot.Version)
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

func (resolver *mutationResolver) CheckoutCartCod(ctx context.Context, quoteID, cartVersion string) (*generated.CODCheckoutResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, errors.New("unauthorized")
	}
	expectedVersion, err := parseCheckoutCartVersion(&cartVersion)
	if err != nil {
		return nil, err
	}
	cartSnapshot, err := resolver.server.cartClient.PrepareCheckoutAtVersion(ctx, uint64(accountID), expectedVersion)
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
	createdOrder, err := resolver.server.orderClient.PostOrderWithQuote(ctx, uint64(accountID), orderProducts, true, quoteID, cartSnapshot.Version)
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
	if err := resolver.server.cartClient.CompleteCheckout(ctx, uint64(accountID)); err != nil {
		return nil, err
	}
	return &generated.CODCheckoutResponse{OrderID: int(createdOrder.ID), Status: codCheckoutStatus}, nil
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
	_ = checksum // ProductMedia does not expose a checksum field yet.
	return toGeneratedProduct(product), nil
}

func (resolver *mutationResolver) AdminUploadKnowledgeDocument(ctx context.Context, file graphql.Upload, idempotencyKey string, productID *string) (*generated.KnowledgeIngestionJob, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	if len(idempotencyKey) == 0 || len(idempotencyKey) > 128 {
		return nil, ErrInvalidParameter
	}
	if !isSupportedKnowledgeDocument(file.Filename) {
		return nil, ErrInvalidParameter
	}
	linkedProductID := ""
	if productID != nil {
		linkedProductID = strings.TrimSpace(*productID)
		if linkedProductID == "" {
			return nil, ErrInvalidParameter
		}
		if _, err := resolver.server.productClient.GetProduct(ctx, linkedProductID); err != nil {
			return nil, ErrInvalidParameter
		}
	}
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return nil, err
	}
	if resolver.server.mediaStore == nil {
		return nil, errors.New("private GCS document storage is not configured")
	}
	uploadCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	uri, checksum, name, err := resolver.server.mediaStore.uploadKnowledgeDocument(uploadCtx, file)
	if err != nil {
		return nil, err
	}
	sourceID := uuid.NewString()
	job, err := resolver.server.recommenderClient.EnqueueKnowledgeIngestion(uploadCtx, sourceID, name, uri,
		strings.Split(file.ContentType, ";")[0], checksum, int64(accountID), idempotencyKey, linkedProductID)
	if err != nil {
		resolver.server.mediaStore.deleteKnowledgeUpload(uploadCtx, uri)
		return nil, err
	}
	if job.SourceId != sourceID { // Replayed uploads reuse the original queued source.
		resolver.server.mediaStore.deleteKnowledgeUpload(uploadCtx, uri)
	}
	return toGeneratedKnowledgeJob(job), nil
}

func (resolver *mutationResolver) AdminPreviewKnowledgeDocument(ctx context.Context, file graphql.Upload) (*generated.KnowledgeDocumentPreview, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return nil, errors.New("forbidden")
	}
	if file.File == nil || file.Size <= 0 || file.Size > maxKnowledgeDocumentBytes || !isSupportedKnowledgeDocument(file.Filename) {
		return nil, ErrInvalidParameter
	}
	data, err := io.ReadAll(io.LimitReader(file.File, maxKnowledgeDocumentBytes+1))
	if err != nil || len(data) != int(file.Size) || len(data) > maxKnowledgeDocumentBytes {
		return nil, ErrInvalidParameter
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if err := validateKnowledgeDocumentBytes(ext, data); err != nil {
		return nil, ErrInvalidParameter
	}
	previewCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	preview, err := resolver.server.recommenderClient.PreviewKnowledgeDocument(previewCtx, filepath.Base(file.Filename), file.ContentType, data)
	if err != nil {
		return nil, err
	}
	samples := make([]*generated.KnowledgeDocumentChunkPreview, 0, len(preview.Samples))
	for _, sample := range preview.Samples {
		samples = append(samples, &generated.KnowledgeDocumentChunkPreview{Index: int(sample.Index), Text: sample.Text, Characters: int(sample.Characters)})
	}
	return &generated.KnowledgeDocumentPreview{
		ChunkCount: int(preview.ChunkCount), TotalCharacters: int(preview.TotalCharacters),
		EstimatedEmbeddingTokens: int(preview.EstimatedEmbeddingTokens), EstimatedEmbeddingCostVnd: int(preview.EstimatedEmbeddingCostVnd),
		CostEstimateAvailable: preview.CostEstimateAvailable, Samples: samples,
	}, nil
}

func (resolver *mutationResolver) AdminCancelKnowledgeIngestionJob(ctx context.Context, id string) (bool, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return false, errors.New("forbidden")
	}
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return false, err
	}
	return resolver.server.recommenderClient.CancelKnowledgeIngestionJob(ctx, id, int64(accountID))
}

func (resolver *mutationResolver) AdminRetryKnowledgeIngestionJob(ctx context.Context, id string) (bool, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return false, errors.New("forbidden")
	}
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return false, err
	}
	return resolver.server.recommenderClient.RetryKnowledgeIngestionJob(ctx, id, int64(accountID))
}

func (resolver *mutationResolver) AdminSetKnowledgeSourceStatus(ctx context.Context, sourceID, status string) (bool, error) {
	if !auth.HasAnyRole(ctx, "operations_admin", "platform_admin") {
		return false, errors.New("forbidden")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "published" && status != "revoked" {
		return false, ErrInvalidParameter
	}
	accountID, err := auth.GetUserIdInt(ctx, true)
	if err != nil {
		return false, err
	}
	return resolver.server.recommenderClient.SetKnowledgeSourceStatus(ctx, sourceID, status, int64(accountID))
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

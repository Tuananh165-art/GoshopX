package graph

import (
	"github.com/99designs/gqlgen/graphql"

	account "github.com/Tuananh165art/GoshopX/account/client"
	admin "github.com/Tuananh165art/GoshopX/admin/client"
	cart "github.com/Tuananh165art/GoshopX/cart/client"
	"github.com/Tuananh165art/GoshopX/graphql/generated"
	inventory "github.com/Tuananh165art/GoshopX/inventory/client"
	notification "github.com/Tuananh165art/GoshopX/notification/client"
	order "github.com/Tuananh165art/GoshopX/order/client"
	payment "github.com/Tuananh165art/GoshopX/payment/client"
	product "github.com/Tuananh165art/GoshopX/product/client"
	recommender "github.com/Tuananh165art/GoshopX/recommender/client"
)

type Server struct {
	accountClient      *account.Client
	productClient      *product.Client
	orderClient        *order.Client
	paymentClient      *payment.Client
	recommenderClient  *recommender.Client
	inventoryClient    *inventory.Client
	cartClient         *cart.Client
	notificationClient *notification.Client
	adminClient        *admin.Client
	mediaStore         *mediaStore
}

func NewGraphQLServer(accountURL, productURL, orderURL, paymentURL, recommenderURL, inventoryURL, cartURL, notificationURL, adminURL string) (*Server, error) {
	accClient, err := account.NewClient(accountURL)
	if err != nil {
		return nil, err
	}

	prodClient, err := product.NewClient(productURL)
	if err != nil {
		accClient.Close()
		return nil, err
	}

	ordClient, err := order.NewClient(orderURL)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		return nil, err
	}

	paymentClient, err := payment.NewClient(paymentURL)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		ordClient.Close()
		return nil, err
	}

	recClient, err := recommender.NewClient(recommenderURL)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		ordClient.Close()
		paymentClient.Close()
		return nil, err
	}

	inventoryClient, err := inventory.NewClient(inventoryURL)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		ordClient.Close()
		paymentClient.Close()
		recClient.Close()
		return nil, err
	}

	cartClient, err := cart.NewClient(cartURL)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		ordClient.Close()
		paymentClient.Close()
		recClient.Close()
		inventoryClient.Close()
		return nil, err
	}

	notificationClient, err := notification.NewClient(notificationURL)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		ordClient.Close()
		paymentClient.Close()
		recClient.Close()
		inventoryClient.Close()
		cartClient.Close()
		return nil, err
	}
	adminClient, err := admin.NewClient(adminURL)
	if err != nil {
		notificationClient.Close()
		cartClient.Close()
		inventoryClient.Close()
		recClient.Close()
		paymentClient.Close()
		ordClient.Close()
		prodClient.Close()
		accClient.Close()
		return nil, err
	}
	store, err := newMediaStore()
	if err != nil {
		adminClient.Close()
		notificationClient.Close()
		cartClient.Close()
		inventoryClient.Close()
		recClient.Close()
		paymentClient.Close()
		ordClient.Close()
		prodClient.Close()
		accClient.Close()
		return nil, err
	}

	return &Server{
		accountClient:      accClient,
		productClient:      prodClient,
		orderClient:        ordClient,
		paymentClient:      paymentClient,
		recommenderClient:  recClient,
		inventoryClient:    inventoryClient,
		cartClient:         cartClient,
		notificationClient: notificationClient,
		adminClient:        adminClient,
		mediaStore:         store,
	}, nil
}

func (server *Server) ToExecutableSchema() graphql.ExecutableSchema {
	return generated.NewExecutableSchema(generated.Config{
		Resolvers: server,
	})
}

func (server *Server) Mutation() generated.MutationResolver {
	return &mutationResolver{server: server}
}

func (server *Server) Query() generated.QueryResolver {
	return &queryResolver{server: server}
}

func (server *Server) Account() generated.AccountResolver {
	return &accountResolver{server: server}
}

func (server *Server) Product() generated.ProductResolver {
	return &productResolver{server: server}
}

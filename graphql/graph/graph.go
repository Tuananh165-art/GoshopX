package graph

import (
	"github.com/99designs/gqlgen/graphql"

	account "github.com/rasadov/EcommerceAPI/account/client"
	"github.com/rasadov/EcommerceAPI/graphql/generated"
	order "github.com/rasadov/EcommerceAPI/order/client"
	payment "github.com/rasadov/EcommerceAPI/payment/client"
	product "github.com/rasadov/EcommerceAPI/product/client"
	recommender "github.com/rasadov/EcommerceAPI/recommender/client"
)

type Server struct {
	accountClient     *account.Client
	productClient     *product.Client
	orderClient       *order.Client
	paymentClient     *payment.Client
	recommenderClient *recommender.Client
}

func NewGraphQLServer(accountUrl, productUrl, orderUrl, paymentUrl, recommenderUrl string) (*Server, error) {
	accClient, err := account.NewClient(accountUrl)
	if err != nil {
		return nil, err
	}

	prodClient, err := product.NewClient(productUrl)
	if err != nil {
		accClient.Close()
		return nil, err
	}

	ordClient, err := order.NewClient(orderUrl)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		return nil, err
	}

	paymentClient, err := payment.NewClient(paymentUrl)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		ordClient.Close()
	}

	recClient, err := recommender.NewClient(recommenderUrl)
	if err != nil {
		accClient.Close()
		prodClient.Close()
		ordClient.Close()
		paymentClient.Close()
		return nil, err
	}

	return &Server{
		accountClient:     accClient,
		productClient:     prodClient,
		orderClient:       ordClient,
		paymentClient:     paymentClient,
		recommenderClient: recClient,
	}, nil
}

func (server *Server) Mutation() generated.MutationResolver {
	return &mutationResolver{
		server: server,
	}
}

func (server *Server) Query() generated.QueryResolver {
	return &queryResolver{
		server: server,
	}
}

func (server *Server) Account() generated.AccountResolver {
	return &accountResolver{
		server: server,
	}
}

func (server *Server) ToExecutableSchema() graphql.ExecutableSchema {
	return generated.NewExecutableSchema(generated.Config{
		Resolvers: server,
	})
}

package main

import (
	"log"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/rasadov/EcommerceAPI/graphql/config"
	"github.com/rasadov/EcommerceAPI/graphql/graph"
	"github.com/rasadov/EcommerceAPI/pkg/middleware"
)

func main() {
	server, err := graph.NewGraphQLServer(config.AccountUrl, config.ProductUrl, config.OrderUrl, config.PaymentUrl, config.RecommenderUrl)
	if err != nil {
		log.Fatal(err)
	}

	srv := handler.New(server.ToExecutableSchema())
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	engine := gin.Default()

	engine.Use(middleware.GinContextToContextMiddleware())

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "It works",
		})
	})
	engine.POST("/graphql",
		middleware.AuthorizeJWT(),
		gin.WrapH(srv),
	)
	engine.GET("/playground", gin.WrapH(playground.Handler("Playground", "/graphql")))

	log.Fatal(engine.Run(":8080"))
}

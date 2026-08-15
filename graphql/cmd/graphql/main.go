package main

import (
	"context"
	"log"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/Tuananh165art/GoshopX/graphql/config"
	"github.com/Tuananh165art/GoshopX/graphql/graph"
	"github.com/Tuananh165art/GoshopX/pkg/middleware"
	"github.com/Tuananh165art/GoshopX/pkg/observability"
	"github.com/gin-gonic/gin"
)

func main() {
	shutdownTracing, traceErr := observability.ConfigureTracing(context.Background(), "graphql")
	if traceErr != nil {
		log.Printf("OpenTelemetry tracing disabled: %v", traceErr)
	} else {
		defer func() { _ = shutdownTracing(context.Background()) }()
	}
	observability.StartMetricsServer(9090)
	server, err := graph.NewGraphQLServer(
		config.AccountUrl,
		config.ProductUrl,
		config.OrderUrl,
		config.PaymentUrl,
		config.RecommenderUrl,
		config.InventoryUrl,
		config.CartUrl,
		config.NotificationUrl,
		config.AdminUrl,
	)
	if err != nil {
		log.Fatal(err)
	}

	srv := handler.New(server.ToExecutableSchema())
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	engine := gin.Default()

	engine.Use(observability.GinMiddleware("graphql"), middleware.GinContextToContextMiddleware(), middleware.CaptureClientIP())

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

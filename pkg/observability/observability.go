// Package observability provides the shared, low-overhead telemetry wiring for
// GoshopX services. It deliberately keeps transport instrumentation separate
// from domain logic so the public GraphQL and internal gRPC boundaries remain
// unchanged.
package observability

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	grpcRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "goshopx",
		Subsystem: "grpc",
		Name:      "requests_total",
		Help:      "Number of completed internal gRPC requests.",
	}, []string{"service", "method", "code"})
	grpcDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "goshopx",
		Subsystem: "grpc",
		Name:      "request_duration_seconds",
		Help:      "Duration of completed internal gRPC requests.",
	}, []string{"service", "method", "code"})
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "goshopx",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Number of completed public HTTP requests.",
	}, []string{"service", "method", "route", "status"})
	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "goshopx",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Duration of completed public HTTP requests.",
	}, []string{"service", "method", "route", "status"})
)

func init() {
	prometheus.MustRegister(grpcRequests, grpcDuration, httpRequests, httpDuration)
}

// ConfigureTracing creates an OTLP/gRPC exporter only when an endpoint is
// configured. This makes tracing opt-in and avoids blocking a service's normal
// startup while Jaeger is unavailable.
func ConfigureTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	if configuredName := os.Getenv("OTEL_SERVICE_NAME"); configuredName != "" {
		serviceName = configuredName
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	provider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes("", attribute.String("service.name", serviceName))),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

// GRPCServerOptions instruments every unary internal service call and exports
// stable Prometheus metrics from the service's dedicated /metrics endpoint.
func GRPCServerOptions(serviceName string) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			started := time.Now()
			response, err := handler(ctx, request)
			code := status.Code(err)
			labels := prometheus.Labels{"service": serviceName, "method": info.FullMethod, "code": code.String()}
			grpcRequests.With(labels).Inc()
			grpcDuration.With(labels).Observe(time.Since(started).Seconds())
			return response, err
		}),
	}
}

// GRPCClientOptions propagates W3C trace context to internal gRPC clients.
func GRPCClientOptions() []grpc.DialOption {
	return []grpc.DialOption{grpc.WithStatsHandler(otelgrpc.NewClientHandler())}
}

// StartMetricsServer exposes Go runtime/process metrics and the GoshopX HTTP
// and gRPC metrics on a pod-local port. Kubernetes ServiceMonitors scrape it;
// it is never an internet-facing endpoint.
func StartMetricsServer(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
			log.Printf("metrics listener on port %d stopped: %v", port, err)
		}
	}()
}

// GinMiddleware traces GraphQL HTTP requests and records their outcome without
// changing GraphQL authorization or routing behavior.
func GinMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		ctx, span := otel.Tracer(serviceName).Start(c.Request.Context(), c.Request.Method+" "+route)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		started := time.Now()
		c.Next()
		statusCode := fmt.Sprintf("%d", c.Writer.Status())
		span.SetAttributes(
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", c.Writer.Status()),
		)
		labels := prometheus.Labels{"service": serviceName, "method": c.Request.Method, "route": route, "status": statusCode}
		httpRequests.With(labels).Inc()
		httpDuration.With(labels).Observe(time.Since(started).Seconds())
	}
}

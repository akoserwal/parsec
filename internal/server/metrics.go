package server

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/stats"
)

// GRPCStatsHandler returns a gRPC stats.Handler that records standard
// rpc.server.* OTel metrics using the given MeterProvider.
// Tracing is disabled (noop provider) since we only want metrics here.
func GRPCStatsHandler(mp metric.MeterProvider) stats.Handler {
	if mp == nil {
		mp = noop.NewMeterProvider()
	}
	return otelgrpc.NewServerHandler(
		otelgrpc.WithMeterProvider(mp),
		otelgrpc.WithTracerProvider(tracenoop.NewTracerProvider()),
	)
}

// MetricsHTTPMiddleware wraps an http.Handler with OTel HTTP instrumentation
// that records http.server.request.duration and http.server.active_requests.
// Tracing is disabled (noop provider) since we only want metrics here.
func MetricsHTTPMiddleware(mp metric.MeterProvider, next http.Handler) http.Handler {
	if mp == nil {
		return next
	}
	return otelhttp.NewMiddleware("parsec-http",
		otelhttp.WithMeterProvider(mp),
		otelhttp.WithTracerProvider(tracenoop.NewTracerProvider()),
	)(next)
}

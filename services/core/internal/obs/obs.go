// Package obs sets up structured logging, Prometheus metrics, and OpenTelemetry tracing.
package obs

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// NewLogger returns a JSON logger. Every record carries the service and component names.
func NewLogger(component string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(traceHandler{h}).With("service", "gitorgasm", "component", component)
}

// traceHandler adds trace_id and span_id from the context to every record.
type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(a)}
}
func (h traceHandler) WithGroup(n string) slog.Handler { return traceHandler{h.Handler.WithGroup(n)} }

// InitTracing installs the W3C propagator and, when endpoint is set, an OTLP/HTTP exporter.
// Without an endpoint, spans are still created so trace ids reach logs and outgoing requests.
func InitTracing(ctx context.Context, component, endpoint string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	res := resource.NewSchemaless(semconv.ServiceName("gitorgasm-" + component))
	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if endpoint != "" {
		exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// MetricsServer serves /metrics on a separate port that is not exposed through ingress.
// When ready is set, the port also answers /healthz and /readyz for processes without a
// public port, such as the worker.
func MetricsServer(addr string, ready func(*http.Request) error) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	if ready != nil {
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := ready(r.WithContext(ctx)); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
	}
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}

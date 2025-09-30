package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// noopExporter is a no-op span exporter for when tracing is disabled
type noopExporter struct{}

func (e *noopExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *noopExporter) Shutdown(ctx context.Context) error {
	return nil
}

// noopMetricExporter is a no-op metric exporter for when metrics are disabled
type noopMetricExporter struct{}

func (e *noopMetricExporter) Export(ctx context.Context, metrics *sdkmetric.ResourceMetrics) error {
	return nil
}

func (e *noopMetricExporter) ForceFlush(ctx context.Context) error {
	return nil
}

func (e *noopMetricExporter) Shutdown(ctx context.Context) error {
	return nil
}

// TelemetryManager manages OpenTelemetry instrumentation
type TelemetryManager struct {
	config         *Config
	logger         *zap.Logger
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	meterProvider  *sdkmetric.MeterProvider
	meter          metric.Meter
}

// NewTelemetryManager creates a new TelemetryManager
func NewTelemetryManager(config *Config, logger *zap.Logger) (*TelemetryManager, error) {
	tm := &TelemetryManager{
		config: config,
		logger: logger,
	}

	// Initialize tracer provider
	if err := tm.initTracerProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracer provider: %w", err)
	}

	// Initialize meter provider
	if err := tm.initMeterProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize meter provider: %w", err)
	}

	// Set global providers
	otel.SetTracerProvider(tm.tracerProvider)
	otel.SetMeterProvider(tm.meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tm.tracer = otel.Tracer(config.OpenTelemetry.ServiceName)
	tm.meter = otel.Meter(config.OpenTelemetry.ServiceName)

	logger.Info("OpenTelemetry initialized successfully",
		zap.String("service_name", config.OpenTelemetry.ServiceName),
		zap.String("exporter_type", config.OpenTelemetry.Tracing.Exporter),
	)

	return tm, nil
}

// initTracerProvider initializes the OpenTelemetry TracerProvider
func (tm *TelemetryManager) initTracerProvider() error {
	res, err := tm.createResource()
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	switch tm.config.OpenTelemetry.Tracing.Exporter {
	case "console":
		exporter, err = stdouttrace.New(
			stdouttrace.WithWriter(os.Stdout),
			stdouttrace.WithPrettyPrint(),
		)
	case "otlp":
		exporter, err = tm.createOTLPExporter()
	case "kafka":
		// For now, use console exporter and send to Kafka separately
		// TODO: Implement direct Kafka exporter
		exporter, err = stdouttrace.New(
			stdouttrace.WithWriter(os.Stdout),
			stdouttrace.WithPrettyPrint(),
		)
	case "none":
		// No-op exporter for when tracing is disabled
		exporter = &noopExporter{}
	default:
		return fmt.Errorf("unsupported exporter type: %s", tm.config.OpenTelemetry.Tracing.Exporter)
	}

	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Configure sampler
	var sampler sdktrace.Sampler
	switch tm.config.OpenTelemetry.Tracing.Sampling.Type {
	case "always_on":
		sampler = sdktrace.AlwaysSample()
	case "always_off":
		sampler = sdktrace.NeverSample()
	case "parentbased_always_on":
		sampler = sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		sampler = sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(tm.config.OpenTelemetry.Tracing.Sampling.Ratio))
	case "traceidratio":
		sampler = sdktrace.TraceIDRatioBased(tm.config.OpenTelemetry.Tracing.Sampling.Ratio)
	default:
		sampler = sdktrace.ParentBased(sdktrace.AlwaysSample()) // Default to parent-based always on
	}

	tm.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	return nil
}

// createOTLPExporter creates an OTLP trace exporter
func (tm *TelemetryManager) createOTLPExporter() (sdktrace.SpanExporter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var client otlptrace.Client
	switch tm.config.OpenTelemetry.Tracing.OTLP.Protocol {
	case "grpc":
		client = otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(tm.config.OpenTelemetry.Tracing.OTLP.Endpoint),
			otlptracegrpc.WithInsecure(),
		)
	case "http":
		client = otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(tm.config.OpenTelemetry.Tracing.OTLP.Endpoint),
			otlptracehttp.WithInsecure(),
		)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", tm.config.OpenTelemetry.Tracing.OTLP.Protocol)
	}

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	return exporter, nil
}

// initMeterProvider initializes the OpenTelemetry MeterProvider
func (tm *TelemetryManager) initMeterProvider() error {
	if !tm.config.OpenTelemetry.Metrics.Enabled {
		// Use no-op meter provider when metrics are disabled
		tm.meterProvider = sdkmetric.NewMeterProvider()
		return nil
	}

	res, err := tm.createResource()
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	var exporter sdkmetric.Exporter
	switch tm.config.OpenTelemetry.Metrics.Exporter {
	case "otlp":
		exporter, err = tm.createOTLPMetricExporter()
	case "kafka":
		// For now, use no-op exporter and send to Kafka separately
		// TODO: Implement direct Kafka metric exporter
		exporter = &noopMetricExporter{}
	case "none":
		exporter = &noopMetricExporter{}
	default:
		return fmt.Errorf("unsupported metrics exporter type: %s", tm.config.OpenTelemetry.Metrics.Exporter)
	}

	if err != nil {
		return fmt.Errorf("failed to create metric exporter: %w", err)
	}

	tm.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(tm.config.OpenTelemetry.Metrics.Interval))),
	)

	return nil
}

// createOTLPMetricExporter creates an OTLP metric exporter
func (tm *TelemetryManager) createOTLPMetricExporter() (sdkmetric.Exporter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var client otlpmetric.Client
	switch tm.config.OpenTelemetry.Metrics.OTLP.Protocol {
	case "grpc":
		client = otlpmetricgrpc.NewClient(
			otlpmetricgrpc.WithEndpoint(tm.config.OpenTelemetry.Metrics.OTLP.Endpoint),
			otlpmetricgrpc.WithInsecure(),
		)
	case "http":
		client = otlpmetrichttp.NewClient(
			otlpmetrichttp.WithEndpoint(tm.config.OpenTelemetry.Metrics.OTLP.Endpoint),
			otlpmetrichttp.WithInsecure(),
		)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", tm.config.OpenTelemetry.Metrics.OTLP.Protocol)
	}

	exporter, err := otlpmetric.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	return exporter, nil
}

// createResource creates the OpenTelemetry resource
func (tm *TelemetryManager) createResource() (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", tm.config.OpenTelemetry.ServiceName),
		attribute.String("service.version", tm.config.OpenTelemetry.ServiceVersion),
		attribute.String("deployment.environment", tm.config.OpenTelemetry.Environment),
	}

	// Add custom resource attributes
	for _, attr := range tm.config.OpenTelemetry.Resource.Attributes {
		attrs = append(attrs, attribute.String(attr.Key, attr.Value))
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(attrs...),
	)
	return res, err
}

// Shutdown shuts down the tracer and meter providers
func (tm *TelemetryManager) Shutdown(ctx context.Context) {
	if tm.tracerProvider != nil {
		if err := tm.tracerProvider.Shutdown(ctx); err != nil {
			tm.logger.Error("Failed to shutdown tracer provider", zap.Error(err))
		}
	}
	if tm.meterProvider != nil {
		if err := tm.meterProvider.Shutdown(ctx); err != nil {
			tm.logger.Error("Failed to shutdown meter provider", zap.Error(err))
		}
	}
}

// CreateSpan creates a new span
func (tm *TelemetryManager) CreateSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tm.tracer.Start(ctx, spanName, opts...)
}

// GetMeter returns the meter for creating metrics
func (tm *TelemetryManager) GetMeter() metric.Meter {
	return tm.meter
}

// LogWithTraceContext logs a message with trace and span IDs
func (tm *TelemetryManager) LogWithTraceContext(ctx context.Context, level zapcore.Level, msg string, fields ...zap.Field) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			fields = append(fields,
				zap.String("trace_id", spanContext.TraceID().String()),
				zap.String("span_id", spanContext.SpanID().String()),
			)
		}
	}

	switch level {
	case zap.DebugLevel:
		tm.logger.Debug(msg, fields...)
	case zap.InfoLevel:
		tm.logger.Info(msg, fields...)
	case zap.WarnLevel:
		tm.logger.Warn(msg, fields...)
	case zap.ErrorLevel:
		tm.logger.Error(msg, fields...)
	case zap.FatalLevel:
		tm.logger.Fatal(msg, fields...)
	default:
		tm.logger.Info(msg, fields...)
	}
}

// StartTelemetryServer starts a simple HTTP server for exposing OpenTelemetry metrics (if needed)
func (tm *TelemetryManager) StartTelemetryServer() {
	// Currently, this is a placeholder. Prometheus exporter for metrics can be added here.
	// For now, the OTel Collector handles receiving metrics.
	tm.logger.Info("Telemetry server placeholder started")
}

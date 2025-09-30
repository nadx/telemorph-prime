package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Load configuration
	config, err := LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Override Kafka brokers from environment if set
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		config.Kafka.Brokers = []string{brokers}
	}

	// Initialize logger
	logger, err := createLogger(config.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Initialize global telemetry
	// initGlobalTelemetry() // Temporarily disabled

	// Initialize telemetry manager
	telemetryManager, err := NewTelemetryManager(config, logger)
	if err != nil {
		logger.Fatal("Failed to initialize telemetry manager", zap.Error(err))
	}
	defer telemetryManager.Shutdown(context.Background())

	// Start telemetry server
	telemetryManager.StartTelemetryServer()

	// Create root span for service startup
	ctx, span := telemetryManager.CreateSpan(context.Background(), "service.startup")
	defer span.End()

	telemetryManager.LogWithTraceContext(ctx, zap.InfoLevel, "Starting Telemorph-Prime Ingestion Service",
		zap.String("service_name", config.OpenTelemetry.ServiceName),
		zap.String("service_version", config.OpenTelemetry.ServiceVersion),
		zap.String("environment", config.OpenTelemetry.Environment),
	)

	// Initialize Kafka producer with tracing
	kafkaProducer, err := NewKafkaProducerWithTracing(config, logger, telemetryManager)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to initialize Kafka producer")
		logger.Fatal("Failed to initialize Kafka producer", zap.Error(err))
	}
	defer kafkaProducer.Close()

	// Start health check server with tracing
	go startHealthServerWithTracing(config, logger, telemetryManager)

	// Start HTTP OTLP server with tracing
	go startHTTPOTLPServerWithTracing(config, kafkaProducer, logger, telemetryManager)

	telemetryManager.LogWithTraceContext(ctx, zap.InfoLevel, "Ingestion service started successfully",
		zap.String("grpc_endpoint", config.Server.GRPCEndpoint),
		zap.String("http_endpoint", config.Server.HTTPEndpoint),
		zap.String("health_endpoint", config.Server.HealthEndpoint),
	)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	telemetryManager.LogWithTraceContext(ctx, zap.InfoLevel, "Shutting down ingestion service...")
	telemetryManager.LogWithTraceContext(ctx, zap.InfoLevel, "Ingestion service stopped")
}

// createLogger creates a logger based on configuration
func createLogger(config LoggingConfig) (*zap.Logger, error) {
	var zapConfig zap.Config

	if config.Development {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	// Set log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	// Set encoding
	if config.Format == "console" {
		zapConfig.Encoding = "console"
	} else {
		zapConfig.Encoding = "json"
	}

	// Set sampling
	zapConfig.Sampling = &zap.SamplingConfig{
		Initial:    config.Sampling.Initial,
		Thereafter: config.Sampling.Thereafter,
	}

	return zapConfig.Build()
}

// startHealthServerWithTracing starts the health check HTTP server with tracing
func startHealthServerWithTracing(config *Config, logger *zap.Logger, tm *TelemetryManager) {
	mux := http.NewServeMux()

	// Health check endpoint with tracing
	mux.HandleFunc(config.Health.Endpoint, func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tm.CreateSpan(r.Context(), "health.check",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
			),
		)
		defer span.End()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   config.OpenTelemetry.ServiceName,
			"version":   config.OpenTelemetry.ServiceVersion,
		}

		json.NewEncoder(w).Encode(response)

		span.SetAttributes(
			attribute.Int("http.status_code", http.StatusOK),
			attribute.String("health.status", "healthy"),
		)

		tm.LogWithTraceContext(ctx, zap.InfoLevel, "Health check completed")
	})

	// Readiness check endpoint with tracing
	mux.HandleFunc(config.Health.ReadinessEndpoint, func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tm.CreateSpan(r.Context(), "health.readiness",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
			),
		)
		defer span.End()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := map[string]interface{}{
			"status":    "ready",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   config.OpenTelemetry.ServiceName,
		}

		json.NewEncoder(w).Encode(response)

		span.SetAttributes(
			attribute.Int("http.status_code", http.StatusOK),
			attribute.String("health.status", "ready"),
		)

		tm.LogWithTraceContext(ctx, zap.InfoLevel, "Readiness check completed")
	})

	// Metrics endpoint with tracing
	mux.HandleFunc(config.Health.MetricsEndpoint, func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tm.CreateSpan(r.Context(), "health.metrics",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
			),
		)
		defer span.End()

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		metrics := fmt.Sprintf(`# Telemorph-Prime Ingestion Service Metrics
# Service: %s
# Version: %s
# Environment: %s
# Timestamp: %s
`, config.OpenTelemetry.ServiceName, config.OpenTelemetry.ServiceVersion,
			config.OpenTelemetry.Environment, time.Now().UTC().Format(time.RFC3339))

		fmt.Fprint(w, metrics)

		span.SetAttributes(
			attribute.Int("http.status_code", http.StatusOK),
			attribute.String("metrics.type", "prometheus"),
		)

		tm.LogWithTraceContext(ctx, zap.InfoLevel, "Metrics endpoint accessed")
	})

	// Wrap mux with OpenTelemetry HTTP instrumentation
	handler := otelhttp.NewHandler(mux, "health-server",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		}),
	)

	server := &http.Server{
		Addr:         config.Server.HealthEndpoint,
		Handler:      handler,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
	}

	logger.Info("Health server starting", zap.String("endpoint", config.Server.HealthEndpoint))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Health server failed", zap.Error(err))
	}
}

// startHTTPOTLPServerWithTracing starts a simple HTTP server for OTLP data with tracing
func startHTTPOTLPServerWithTracing(config *Config, kafkaProducer *KafkaProducer, logger *zap.Logger, tm *TelemetryManager) {
	mux := http.NewServeMux()

	// OTLP traces endpoint with tracing
	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tm.CreateSpan(r.Context(), "otlp.traces.receive",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.Int("http.request.content_length", int(r.ContentLength)),
				attribute.String("otlp.signal", "traces"),
			),
		)
		defer span.End()

		if r.Method != http.MethodPost {
			span.SetStatus(codes.Error, "Method not allowed")
			span.SetAttributes(attribute.Int("http.status_code", http.StatusMethodNotAllowed))
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var tracesData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&tracesData); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Invalid JSON")
			span.SetAttributes(attribute.Int("http.status_code", http.StatusBadRequest))
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Process traces data with tracing
		ctx, processSpan := tm.CreateSpan(ctx, "otlp.traces.process")
		tm.LogWithTraceContext(ctx, zap.InfoLevel, "Received traces data",
			zap.Any("data", tracesData),
			zap.String("signal_type", "traces"),
		)

		// Send traces data to Kafka
		if err := kafkaProducer.SendMessageWithTracing(ctx, config.Kafka.Topics.Traces, "traces", tracesData, map[string]string{
			"signal_type":  "traces",
			"content_type": "application/json",
		}); err != nil {
			processSpan.RecordError(err)
			processSpan.SetStatus(codes.Error, "Failed to send traces to Kafka")
			tm.LogWithTraceContext(ctx, zap.ErrorLevel, "Failed to send traces to Kafka", zap.Error(err))
		} else {
			processSpan.SetStatus(codes.Ok, "Traces sent to Kafka successfully")
		}
		processSpan.End()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

		span.SetAttributes(
			attribute.Int("http.status_code", http.StatusOK),
			attribute.String("otlp.signal", "traces"),
		)
	})

	// OTLP metrics endpoint with tracing
	mux.HandleFunc("/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tm.CreateSpan(r.Context(), "otlp.metrics.receive",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.Int("http.request.content_length", int(r.ContentLength)),
				attribute.String("otlp.signal", "metrics"),
			),
		)
		defer span.End()

		if r.Method != http.MethodPost {
			span.SetStatus(codes.Error, "Method not allowed")
			span.SetAttributes(attribute.Int("http.status_code", http.StatusMethodNotAllowed))
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var metricsData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&metricsData); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Invalid JSON")
			span.SetAttributes(attribute.Int("http.status_code", http.StatusBadRequest))
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Process metrics data with tracing
		ctx, processSpan := tm.CreateSpan(ctx, "otlp.metrics.process")
		tm.LogWithTraceContext(ctx, zap.InfoLevel, "Received metrics data",
			zap.Any("data", metricsData),
			zap.String("signal_type", "metrics"),
		)

		// Send metrics data to Kafka
		if err := kafkaProducer.SendMessageWithTracing(ctx, config.Kafka.Topics.Metrics, "metrics", metricsData, map[string]string{
			"signal_type":  "metrics",
			"content_type": "application/json",
		}); err != nil {
			processSpan.RecordError(err)
			processSpan.SetStatus(codes.Error, "Failed to send metrics to Kafka")
			tm.LogWithTraceContext(ctx, zap.ErrorLevel, "Failed to send metrics to Kafka", zap.Error(err))
		} else {
			processSpan.SetStatus(codes.Ok, "Metrics sent to Kafka successfully")
		}
		processSpan.End()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

		span.SetAttributes(
			attribute.Int("http.status_code", http.StatusOK),
			attribute.String("otlp.signal", "metrics"),
		)
	})

	// OTLP logs endpoint with tracing
	mux.HandleFunc("/v1/logs", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tm.CreateSpan(r.Context(), "otlp.logs.receive",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.Int("http.request.content_length", int(r.ContentLength)),
				attribute.String("otlp.signal", "logs"),
			),
		)
		defer span.End()

		if r.Method != http.MethodPost {
			span.SetStatus(codes.Error, "Method not allowed")
			span.SetAttributes(attribute.Int("http.status_code", http.StatusMethodNotAllowed))
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var logsData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&logsData); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Invalid JSON")
			span.SetAttributes(attribute.Int("http.status_code", http.StatusBadRequest))
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Process logs data with tracing
		ctx, processSpan := tm.CreateSpan(ctx, "otlp.logs.process")
		tm.LogWithTraceContext(ctx, zap.InfoLevel, "Received logs data",
			zap.Any("data", logsData),
			zap.String("signal_type", "logs"),
		)

		// Send logs data to Kafka
		if err := kafkaProducer.SendMessageWithTracing(ctx, config.Kafka.Topics.Logs, "logs", logsData, map[string]string{
			"signal_type":  "logs",
			"content_type": "application/json",
		}); err != nil {
			processSpan.RecordError(err)
			processSpan.SetStatus(codes.Error, "Failed to send logs to Kafka")
			tm.LogWithTraceContext(ctx, zap.ErrorLevel, "Failed to send logs to Kafka", zap.Error(err))
		} else {
			processSpan.SetStatus(codes.Ok, "Logs sent to Kafka successfully")
		}
		processSpan.End()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

		span.SetAttributes(
			attribute.Int("http.status_code", http.StatusOK),
			attribute.String("otlp.signal", "logs"),
		)
	})

	// Wrap mux with OpenTelemetry HTTP instrumentation
	handler := otelhttp.NewHandler(mux, "otlp-server",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		}),
	)

	server := &http.Server{
		Addr:         config.Server.HTTPEndpoint,
		Handler:      handler,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
	}

	logger.Info("HTTP OTLP server starting", zap.String("endpoint", config.Server.HTTPEndpoint))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP OTLP server failed", zap.Error(err))
	}
}

// KafkaProducer handles Kafka message production with tracing
type KafkaProducer struct {
	producer         sarama.SyncProducer
	logger           *zap.Logger
	telemetryManager *TelemetryManager
	config           *Config
}

// NewKafkaProducerWithTracing creates a new Kafka producer with tracing
func NewKafkaProducerWithTracing(config *Config, logger *zap.Logger, tm *TelemetryManager) (*KafkaProducer, error) {
	ctx, span := tm.CreateSpan(context.Background(), "kafka.producer.init")
	defer span.End()

	saramaConfig := sarama.NewConfig()

	// Configure producer settings
	switch config.Kafka.Producer.RequiredAcks {
	case "WaitForAll":
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	case "WaitForLocal":
		saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal
	case "NoResponse":
		saramaConfig.Producer.RequiredAcks = sarama.NoResponse
	default:
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	}

	saramaConfig.Producer.Retry.Max = config.Kafka.Producer.RetryMax
	saramaConfig.Producer.Return.Successes = true

	// Configure compression
	switch config.Kafka.Producer.Compression {
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	}

	saramaConfig.Producer.Flush.Bytes = config.Kafka.Producer.BatchSize
	saramaConfig.Producer.Flush.Frequency = config.Kafka.Producer.BatchTimeout

	// Add OpenTelemetry instrumentation
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true

	producer, err := sarama.NewSyncProducer(config.Kafka.Brokers, saramaConfig)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create Kafka producer")
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	// Use producer directly (Kafka instrumentation will be added later)
	instrumentedProducer := producer

	kp := &KafkaProducer{
		producer:         instrumentedProducer,
		logger:           logger,
		telemetryManager: tm,
		config:           config,
	}

	span.SetAttributes(
		attribute.String("kafka.brokers", fmt.Sprintf("%v", config.Kafka.Brokers)),
		attribute.String("kafka.compression", config.Kafka.Producer.Compression),
		attribute.Int("kafka.retry_max", config.Kafka.Producer.RetryMax),
	)

	tm.LogWithTraceContext(ctx, zap.InfoLevel, "Kafka producer initialized successfully",
		zap.Strings("brokers", config.Kafka.Brokers),
		zap.String("compression", config.Kafka.Producer.Compression),
	)

	return kp, nil
}

// Close closes the Kafka producer
func (kp *KafkaProducer) Close() error {
	ctx, span := kp.telemetryManager.CreateSpan(context.Background(), "kafka.producer.close")
	defer span.End()

	err := kp.producer.Close()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to close Kafka producer")
	} else {
		span.SetStatus(codes.Ok, "Kafka producer closed successfully")
	}

	kp.telemetryManager.LogWithTraceContext(ctx, zap.InfoLevel, "Kafka producer closed")
	return err
}

// SendMessageWithTracing sends a message to Kafka with tracing and context propagation
func (kp *KafkaProducer) SendMessageWithTracing(ctx context.Context, topic, key string, value interface{}, headers map[string]string) error {
	spanName := fmt.Sprintf("kafka.produce %s", topic)
	ctx, span := kp.telemetryManager.CreateSpan(ctx, spanName,
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "publish"),
			attribute.String("kafka.topic", topic),
			attribute.String("kafka.key", key),
		),
	)
	defer span.End()

	// Serialize value to JSON
	valueBytes, err := json.Marshal(value)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal value to JSON")
		kp.telemetryManager.LogWithTraceContext(ctx, zap.ErrorLevel, "Failed to marshal value to JSON",
			zap.Error(err),
			zap.String("topic", topic),
			zap.String("key", key),
		)
		return err
	}

	// Create Kafka message
	message := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(valueBytes),
	}

	// Add headers
	for k, v := range headers {
		message.Headers = append(message.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	// Inject trace context into Kafka headers (simplified for now)
	// TODO: Add proper Kafka context propagation

	// Send message
	partition, offset, err := kp.producer.SendMessage(message)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to send message to Kafka")
		kp.telemetryManager.LogWithTraceContext(ctx, zap.ErrorLevel, "Failed to send message to Kafka",
			zap.Error(err),
			zap.String("topic", topic),
			zap.String("key", key),
		)
		return err
	}

	// Add success attributes
	span.SetAttributes(
		attribute.Int("kafka.partition", int(partition)),
		attribute.Int64("kafka.offset", offset),
		attribute.String("kafka.status", "success"),
		attribute.Int("message.size", len(valueBytes)),
	)

	span.SetStatus(codes.Ok, "Message sent successfully")

	kp.telemetryManager.LogWithTraceContext(ctx, zap.InfoLevel, "Message sent to Kafka successfully",
		zap.String("topic", topic),
		zap.String("key", key),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
	)

	return nil
}

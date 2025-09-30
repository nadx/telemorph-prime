package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Kafka         KafkaConfig         `yaml:"kafka"`
	Logging       LoggingConfig       `yaml:"logging"`
	OpenTelemetry OpenTelemetryConfig `yaml:"opentelemetry"`
	Health        HealthConfig        `yaml:"health"`
	Performance   PerformanceConfig   `yaml:"performance"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	GRPCEndpoint   string        `yaml:"grpc_endpoint"`
	HTTPEndpoint   string        `yaml:"http_endpoint"`
	HealthEndpoint string        `yaml:"health_endpoint"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers  []string       `yaml:"brokers"`
	Topics   TopicsConfig   `yaml:"topics"`
	Producer ProducerConfig `yaml:"producer"`
}

// TopicsConfig holds Kafka topic names
type TopicsConfig struct {
	Traces  string `yaml:"traces"`
	Metrics string `yaml:"metrics"`
	Logs    string `yaml:"logs"`
}

// ProducerConfig holds Kafka producer configuration
type ProducerConfig struct {
	RequiredAcks string        `yaml:"required_acks"`
	RetryMax     int           `yaml:"retry_max"`
	Compression  string        `yaml:"compression"`
	BatchSize    int           `yaml:"batch_size"`
	BatchTimeout time.Duration `yaml:"batch_timeout"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level       string         `yaml:"level"`
	Format      string         `yaml:"format"`
	Development bool           `yaml:"development"`
	Sampling    SamplingConfig `yaml:"sampling"`
}

// SamplingConfig holds logging sampling configuration
type SamplingConfig struct {
	Initial    int `yaml:"initial"`
	Thereafter int `yaml:"thereafter"`
}

// OpenTelemetryConfig holds OpenTelemetry configuration
type OpenTelemetryConfig struct {
	ServiceName    string         `yaml:"service_name"`
	ServiceVersion string         `yaml:"service_version"`
	Environment    string         `yaml:"environment"`
	Tracing        TracingConfig  `yaml:"tracing"`
	Metrics        MetricsConfig  `yaml:"metrics"`
	Resource       ResourceConfig `yaml:"resource"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	Enabled  bool                  `yaml:"enabled"`
	Exporter string                `yaml:"exporter"`
	OTLP     OTLPConfig            `yaml:"otlp"`
	Jaeger   JaegerConfig          `yaml:"jaeger"`
	Sampling SamplingTracingConfig `yaml:"sampling"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Exporter string        `yaml:"exporter"`
	OTLP     OTLPConfig    `yaml:"otlp"`
	Interval time.Duration `yaml:"interval"`
}

// OTLPConfig holds OTLP exporter configuration
type OTLPConfig struct {
	Endpoint string `yaml:"endpoint"`
	Protocol string `yaml:"protocol"`
	Insecure bool   `yaml:"insecure"`
}

// JaegerConfig holds Jaeger exporter configuration
type JaegerConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// SamplingTracingConfig holds tracing sampling configuration
type SamplingTracingConfig struct {
	Type  string  `yaml:"type"`
	Ratio float64 `yaml:"ratio"`
}

// ResourceConfig holds resource attributes configuration
type ResourceConfig struct {
	Attributes []AttributeConfig `yaml:"attributes"`
}

// AttributeConfig holds a single resource attribute
type AttributeConfig struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Endpoint          string `yaml:"endpoint"`
	ReadinessEndpoint string `yaml:"readiness_endpoint"`
	LivenessEndpoint  string `yaml:"liveness_endpoint"`
	MetricsEndpoint   string `yaml:"metrics_endpoint"`
}

// PerformanceConfig holds performance-related configuration
type PerformanceConfig struct {
	MaxConcurrentRequests   int           `yaml:"max_concurrent_requests"`
	RequestTimeout          time.Duration `yaml:"request_timeout"`
	GracefulShutdownTimeout time.Duration `yaml:"graceful_shutdown_timeout"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set default config path if not provided
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults for missing values
	setDefaults(&config)

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	// Server defaults
	if config.Server.GRPCEndpoint == "" {
		config.Server.GRPCEndpoint = "0.0.0.0:4317"
	}
	if config.Server.HTTPEndpoint == "" {
		config.Server.HTTPEndpoint = "0.0.0.0:4318"
	}
	if config.Server.HealthEndpoint == "" {
		config.Server.HealthEndpoint = "0.0.0.0:8080"
	}
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 5 * time.Second
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 10 * time.Second
	}

	// Kafka defaults
	if len(config.Kafka.Brokers) == 0 {
		config.Kafka.Brokers = []string{"kafka:29092"}
	}
	if config.Kafka.Topics.Traces == "" {
		config.Kafka.Topics.Traces = "otel.traces"
	}
	if config.Kafka.Topics.Metrics == "" {
		config.Kafka.Topics.Metrics = "otel.metrics"
	}
	if config.Kafka.Topics.Logs == "" {
		config.Kafka.Topics.Logs = "otel.logs"
	}

	// Logging defaults
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}

	// OpenTelemetry defaults
	if config.OpenTelemetry.ServiceName == "" {
		config.OpenTelemetry.ServiceName = "telemorph-ingestion-service"
	}
	if config.OpenTelemetry.ServiceVersion == "" {
		config.OpenTelemetry.ServiceVersion = "1.0.0"
	}
	if config.OpenTelemetry.Environment == "" {
		config.OpenTelemetry.Environment = "development"
	}

	// Health defaults
	if config.Health.Endpoint == "" {
		config.Health.Endpoint = "/health"
	}
	if config.Health.ReadinessEndpoint == "" {
		config.Health.ReadinessEndpoint = "/ready"
	}
	if config.Health.LivenessEndpoint == "" {
		config.Health.LivenessEndpoint = "/live"
	}
	if config.Health.MetricsEndpoint == "" {
		config.Health.MetricsEndpoint = "/metrics"
	}

	// Performance defaults
	if config.Performance.MaxConcurrentRequests == 0 {
		config.Performance.MaxConcurrentRequests = 1000
	}
	if config.Performance.RequestTimeout == 0 {
		config.Performance.RequestTimeout = 30 * time.Second
	}
	if config.Performance.GracefulShutdownTimeout == 0 {
		config.Performance.GracefulShutdownTimeout = 30 * time.Second
	}
}


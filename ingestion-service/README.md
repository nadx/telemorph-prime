# Telemorph-Prime Ingestion Service

The ingestion service is the entry point for all OpenTelemetry telemetry data in the Telemorph-Prime observability platform. It receives traces, metrics, and logs via OTLP (OpenTelemetry Protocol) and forwards them to Apache Kafka for further processing.

## Features

- **OTLP Support**: Receives telemetry data via both gRPC and HTTP OTLP protocols
- **Kafka Integration**: Forwards all telemetry data to Apache Kafka topics
- **Health Monitoring**: Provides health and readiness endpoints
- **Error Handling**: Robust error handling and retry logic
- **JSON Serialization**: Converts OpenTelemetry data to JSON format for Kafka

## Architecture

```
Applications → OTEL SDK → OTEL Collector → Ingestion Service → Kafka Topics
                                                              ├── otel.traces
                                                              ├── otel.metrics
                                                              └── otel.logs
```

## Endpoints

- **gRPC OTLP**: `localhost:4317`
- **HTTP OTLP**: `localhost:4318`
- **Health Check**: `localhost:8080/health`
- **Readiness Check**: `localhost:8080/ready`
- **Metrics**: `localhost:8080/metrics`

## Quick Start

### Using Docker Compose (Recommended)

1. Start all services:
```bash
docker-compose up -d
```

2. Check service health:
```bash
curl http://localhost:8080/health
```

3. Run tests:
```bash
./test-ingestion.sh
```

### Local Development

1. Install Go 1.21+
2. Install dependencies:
```bash
go mod download
```

3. Start Kafka (using Docker):
```bash
docker-compose up -d zookeeper kafka
```

4. Run the service:
```bash
go run .
```

## Configuration

The service uses the following default configuration:

- **Kafka Brokers**: `localhost:9092`
- **gRPC Endpoint**: `0.0.0.0:4317`
- **HTTP Endpoint**: `0.0.0.0:4318`
- **Health Endpoint**: `0.0.0.0:8080`

## Kafka Topics

The service creates and writes to the following Kafka topics:

- **otel.traces**: Trace data (partitioned by trace ID)
- **otel.metrics**: Metric data (partitioned by service name)
- **otel.logs**: Log data (partitioned by service name)

## Message Format

### Trace Messages
```json
{
  "trace_id": "12345678901234567890123456789012",
  "span_id": "1234567890123456",
  "parent_span_id": "1234567890123456",
  "service_name": "my-service",
  "operation_name": "GET /api/users",
  "start_time": 1640995200000000000,
  "duration_nanos": 100000000,
  "attributes": {...},
  "resource_attributes": {...},
  "events": [...],
  "status_code": "OK",
  "ingest_timestamp": 1640995200000000000
}
```

### Metric Messages
```json
{
  "metric_name": "http_requests_total",
  "metric_type": "counter",
  "value": 42.0,
  "timestamp": 1640995200000000000,
  "service_name": "my-service",
  "labels": {...},
  "resource_attributes": {...},
  "ingest_timestamp": 1640995200000000000
}
```

### Log Messages
```json
{
  "trace_id": "12345678901234567890123456789012",
  "span_id": "1234567890123456",
  "service_name": "my-service",
  "log_level": "INFO",
  "message": "User logged in",
  "timestamp": 1640995200000000000,
  "attributes": {...},
  "resource_attributes": {...},
  "ingest_timestamp": 1640995200000000000
}
```

## Testing

### Health Checks
```bash
# Health check
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/ready

# Metrics
curl http://localhost:8080/metrics
```

### Sending Test Data

You can send test telemetry data using curl or any OpenTelemetry SDK. The service accepts standard OTLP format.

Example trace data:
```bash
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"stringValue": "test-service"}
        }]
      },
      "scopeSpans": [{
        "spans": [{
          "traceId": "12345678901234567890123456789012",
          "spanId": "1234567890123456",
          "name": "test-span",
          "startTimeUnixNano": "1640995200000000000",
          "endTimeUnixNano": "1640995201000000000"
        }]
      }]
    }]
  }'
```

## Monitoring

The service provides several monitoring endpoints:

- **Health**: Basic health status
- **Readiness**: Service readiness for traffic
- **Metrics**: Basic service metrics (placeholder)

## Troubleshooting

### Common Issues

1. **Kafka Connection Failed**
   - Ensure Kafka is running: `docker-compose ps`
   - Check Kafka logs: `docker-compose logs kafka`

2. **Port Already in Use**
   - Check if ports 4317, 4318, or 8080 are already in use
   - Stop conflicting services or change ports in docker-compose.yml

3. **Health Check Fails**
   - Check service logs: `docker-compose logs ingestion-service`
   - Verify all dependencies are running

### Logs

View service logs:
```bash
# All logs
docker-compose logs ingestion-service

# Follow logs
docker-compose logs -f ingestion-service
```

## Development

### Project Structure
```
ingestion-service/
├── main.go              # Main server setup and OTLP receivers
├── traces_writer.go     # Trace data processing and Kafka publishing
├── metrics_writer.go    # Metric data processing and Kafka publishing
├── logs_writer.go       # Log data processing and Kafka publishing
├── go.mod              # Go module dependencies
├── Dockerfile          # Container build configuration
└── README.md           # This file
```

### Adding New Features

1. **New Signal Types**: Add new writer files following the existing pattern
2. **New Kafka Topics**: Update topic constants and add topic creation logic
3. **New Endpoints**: Add new HTTP handlers in main.go

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../../LICENSE) file for details.


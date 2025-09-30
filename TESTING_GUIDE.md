# Telemorph-Prime Testing Guide

This guide provides comprehensive instructions for testing the Telemorph-Prime ingestion service to verify that telemetry data is being properly ingested, processed, and is ready for storage.

## Prerequisites

1. **Docker and Docker Compose** installed
2. **Python 3.7+** with pip (for Kafka consumer testing)
3. **curl** and **jq** (for API testing)
4. **Git** (for cloning the repository)

## Quick Start

### 1. Start the System

```bash
# Start all services
docker-compose up -d

# Check if all services are running
docker-compose ps
```

### 2. Run Basic Health Check

```bash
# Check system health
./monitor-system.sh

# Or check specific components
./monitor-system.sh health
./monitor-system.sh kafka
./monitor-system.sh otlp
```

### 3. Send Test Data

```bash
# Send sample telemetry data
./test-telemetry.sh

# Send specific types of data
./test-telemetry.sh traces
./test-telemetry.sh metrics
./test-telemetry.sh logs

# Run continuous testing
./test-telemetry.sh continuous
```

### 4. Verify Data in Kafka

```bash
# Install Python dependencies
pip install -r requirements.txt

# Consume and analyze messages from Kafka
python3 kafka-consumer-test.py

# Consume with custom parameters
python3 kafka-consumer-test.py 20 60  # max 20 messages, 60s timeout
```

## Detailed Testing Procedures

### Phase 1: System Health Verification

#### 1.1 Check Service Health

```bash
# Basic health check
curl http://localhost:8080/health

# Detailed health information
curl http://localhost:8080/health | jq '.'

# Readiness check
curl http://localhost:8080/ready

# Metrics endpoint
curl http://localhost:8080/metrics
```

**Expected Results:**
- All endpoints should return HTTP 200
- Health response should include service information
- Metrics should show Prometheus-format data

#### 1.2 Check OTLP Endpoints

```bash
# Test traces endpoint
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}'

# Test metrics endpoint  
curl -X POST http://localhost:4318/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}'

# Test logs endpoint
curl -X POST http://localhost:4318/v1/logs \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}'
```

**Expected Results:**
- All endpoints should return HTTP 200
- Response should be `{"status": "success"}`

### Phase 2: Data Ingestion Testing

#### 2.1 Send Sample Traces

```bash
# Send comprehensive trace data
./test-telemetry.sh traces
```

**What to Verify:**
- HTTP 200 response
- Trace data structure is valid
- Service logs show trace processing
- Data appears in Kafka `otel.traces` topic

#### 2.2 Send Sample Metrics

```bash
# Send comprehensive metrics data
./test-telemetry.sh metrics
```

**What to Verify:**
- HTTP 200 response
- Metrics data structure is valid
- Service logs show metrics processing
- Data appears in Kafka `otel.metrics` topic

#### 2.3 Send Sample Logs

```bash
# Send comprehensive logs data
./test-telemetry.sh logs
```

**What to Verify:**
- HTTP 200 response
- Logs data structure is valid
- Service logs show logs processing
- Data appears in Kafka `otel.logs` topic

### Phase 3: Kafka Verification

#### 3.1 Check Kafka Topics

```bash
# List all topics
docker exec telemorph-kafka kafka-topics --list --bootstrap-server localhost:9092

# Check specific topic details
docker exec telemorph-kafka kafka-topics --describe --topic otel.traces --bootstrap-server localhost:9092
docker exec telemorph-kafka kafka-topics --describe --topic otel.metrics --bootstrap-server localhost:9092
docker exec telemorph-kafka kafka-topics --describe --topic otel.logs --bootstrap-server localhost:9092
```

**Expected Results:**
- All three topics should exist
- Topics should have 3 partitions each
- Replication factor should be 1

#### 3.2 Consume Messages from Kafka

```bash
# Consume and analyze messages
python3 kafka-consumer-test.py

# Consume with custom parameters
python3 kafka-consumer-test.py 50 120  # 50 messages, 2 minutes timeout
```

**What to Verify:**
- Messages are being consumed from all topics
- Message structure matches OTLP format
- Headers contain signal type and content type
- Data is properly serialized as JSON

#### 3.3 Use Kafka UI

1. Open http://localhost:8081 in your browser
2. Navigate to "Topics" section
3. Click on each topic (`otel.traces`, `otel.metrics`, `otel.logs`)
4. Check "Messages" tab to see recent messages
5. Verify message content and structure

### Phase 4: Continuous Testing

#### 4.1 Run Continuous Data Generation

```bash
# Run continuous testing (sends data every 5 seconds)
./test-telemetry.sh continuous
```

**What to Monitor:**
- Service logs for any errors
- Kafka UI for message flow
- System resource usage
- Response times

#### 4.2 Monitor System Performance

```bash
# Check system resources
./monitor-system.sh resources

# Check container status
./monitor-system.sh containers

# Run full monitoring dashboard
./monitor-system.sh dashboard
```

### Phase 5: Error Handling Testing

#### 5.1 Test Invalid Data

```bash
# Send invalid JSON
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{"invalid": json}'

# Send wrong content type
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: text/plain" \
  -d 'some text'

# Send empty data
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d ''
```

**Expected Results:**
- Invalid requests should return HTTP 400
- Service should log errors appropriately
- Service should continue processing valid requests

#### 5.2 Test Service Resilience

```bash
# Restart Kafka and check service recovery
docker-compose restart kafka
sleep 10
./monitor-system.sh health

# Restart ingestion service
docker-compose restart ingestion-service
sleep 10
./monitor-system.sh health
```

## Verification Checklist

### ✅ Data Ingestion
- [ ] OTLP HTTP endpoints accept data
- [ ] OTLP gRPC endpoints accept data (if implemented)
- [ ] All three signal types (traces, metrics, logs) work
- [ ] Data validation works correctly
- [ ] Error handling works for invalid data

### ✅ Data Processing
- [ ] Data is properly serialized to JSON
- [ ] Trace context is preserved
- [ ] Headers are added correctly
- [ ] Service logs show processing information

### ✅ Data Storage
- [ ] Data appears in Kafka topics
- [ ] Message structure is correct
- [ ] Partitioning works (if configured)
- [ ] Data persistence works

### ✅ System Health
- [ ] Health endpoints respond correctly
- [ ] Metrics endpoint provides data
- [ ] Service recovers from failures
- [ ] Resource usage is reasonable

### ✅ Monitoring
- [ ] Service logs are informative
- [ ] Tracing works throughout the pipeline
- [ ] Metrics are collected
- [ ] System monitoring tools work

## Troubleshooting

### Common Issues

#### Service Not Starting
```bash
# Check logs
docker-compose logs ingestion-service

# Check configuration
cat ingestion-service/config.yaml

# Restart service
docker-compose restart ingestion-service
```

#### Kafka Connection Issues
```bash
# Check Kafka status
docker-compose logs kafka

# Check Kafka topics
docker exec telemorph-kafka kafka-topics --list --bootstrap-server localhost:9092

# Restart Kafka
docker-compose restart kafka
```

#### No Data in Kafka
```bash
# Check if data is being sent
./test-telemetry.sh traces

# Check Kafka consumer
python3 kafka-consumer-test.py 5 10

# Check service logs
docker-compose logs ingestion-service | grep -i kafka
```

### Performance Issues

#### High Memory Usage
```bash
# Check container resources
docker stats

# Check system resources
./monitor-system.sh resources

# Restart services if needed
docker-compose restart
```

#### Slow Response Times
```bash
# Check service logs for errors
docker-compose logs ingestion-service

# Check Kafka performance
docker exec telemorph-kafka kafka-log-dirs --bootstrap-server localhost:9092 --describe
```

## Success Criteria

The ingestion service is ready for Phase 2 when:

1. **All health checks pass** ✅
2. **OTLP endpoints accept data** ✅
3. **Data appears in Kafka topics** ✅
4. **Message structure is correct** ✅
5. **Error handling works** ✅
6. **System is stable under load** ✅
7. **Monitoring is functional** ✅

## Next Steps

Once Phase 1 testing is complete:

1. **Phase 2**: Implement stream processing with Apache Flink
2. **Phase 2**: Add storage writers for Druid and Hudi
3. **Phase 3**: Build query engine and APIs
4. **Phase 4**: Create web frontend
5. **Phase 5**: Add MCP integration

## Additional Resources

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Kafka Documentation](https://kafka.apache.org/documentation/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Project ROADMAP.md](ROADMAP.md)

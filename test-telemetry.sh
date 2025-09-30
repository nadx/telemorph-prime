#!/bin/bash

# Telemorph-Prime Telemetry Testing Script
# This script sends sample telemetry data to test the ingestion service

set -e

# Configuration
INGESTION_SERVICE_URL="http://localhost:4318"
HEALTH_URL="http://localhost:8080"
KAFKA_UI_URL="http://localhost:8081"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Telemorph-Prime Telemetry Testing Script${NC}"
echo "=================================================="

# Function to check if service is running
check_service() {
    echo -e "${YELLOW}📡 Checking ingestion service health...${NC}"
    
    if curl -s -f "$HEALTH_URL/health" > /dev/null; then
        echo -e "${GREEN}✅ Ingestion service is healthy${NC}"
    else
        echo -e "${RED}❌ Ingestion service is not responding${NC}"
        echo "Please ensure the service is running with: docker-compose up"
        exit 1
    fi
}

# Function to send sample traces
send_sample_traces() {
    echo -e "${YELLOW}📊 Sending sample traces...${NC}"
    
    # Sample trace data in OTLP format
    local trace_data='{
        "resourceSpans": [{
            "resource": {
                "attributes": [{
                    "key": "service.name",
                    "value": {"stringValue": "test-service"}
                }, {
                    "key": "service.version", 
                    "value": {"stringValue": "1.0.0"}
                }]
            },
            "scopeSpans": [{
                "scope": {
                    "name": "test-instrumentation",
                    "version": "1.0.0"
                },
                "spans": [{
                    "traceId": "12345678901234567890123456789012",
                    "spanId": "1234567890123456",
                    "name": "test-operation",
                    "kind": "SPAN_KIND_SERVER",
                    "startTimeUnixNano": "'$(date +%s%N)'",
                    "endTimeUnixNano": "'$(($(date +%s%N) + 100000000))'",
                    "attributes": [{
                        "key": "http.method",
                        "value": {"stringValue": "GET"}
                    }, {
                        "key": "http.url",
                        "value": {"stringValue": "/test"}
                    }, {
                        "key": "http.status_code",
                        "value": {"intValue": "200"}
                    }],
                    "status": {
                        "code": "STATUS_CODE_OK"
                    }
                }]
            }]
        }]
    }'
    
    local response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$trace_data" \
        "$INGESTION_SERVICE_URL/v1/traces")
    
    local http_code="${response: -3}"
    if [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✅ Traces sent successfully${NC}"
    else
        echo -e "${RED}❌ Failed to send traces (HTTP $http_code)${NC}"
        echo "Response: ${response%???}"
    fi
}

# Function to send sample metrics
send_sample_metrics() {
    echo -e "${YELLOW}📈 Sending sample metrics...${NC}"
    
    # Sample metrics data in OTLP format
    local metrics_data='{
        "resourceMetrics": [{
            "resource": {
                "attributes": [{
                    "key": "service.name",
                    "value": {"stringValue": "test-service"}
                }]
            },
            "scopeMetrics": [{
                "scope": {
                    "name": "test-instrumentation",
                    "version": "1.0.0"
                },
                "metrics": [{
                    "name": "http_requests_total",
                    "description": "Total HTTP requests",
                    "unit": "1",
                    "sum": {
                        "dataPoints": [{
                            "attributes": [{
                                "key": "method",
                                "value": {"stringValue": "GET"}
                            }, {
                                "key": "status",
                                "value": {"stringValue": "200"}
                            }],
                            "startTimeUnixNano": "'$(date +%s%N)'",
                            "timeUnixNano": "'$(date +%s%N)'",
                            "asInt": "42"
                        }],
                        "aggregationTemporality": "AGGREGATION_TEMPORALITY_CUMULATIVE",
                        "isMonotonic": true
                    }
                }, {
                    "name": "http_request_duration_seconds",
                    "description": "HTTP request duration",
                    "unit": "s",
                    "histogram": {
                        "dataPoints": [{
                            "attributes": [{
                                "key": "method",
                                "value": {"stringValue": "GET"}
                            }],
                            "startTimeUnixNano": "'$(date +%s%N)'",
                            "timeUnixNano": "'$(date +%s%N)'",
                            "count": "10",
                            "sum": 1.5,
                            "bucketCounts": ["0", "5", "3", "2", "0"],
                            "explicitBounds": [0.1, 0.5, 1.0, 2.0]
                        }],
                        "aggregationTemporality": "AGGREGATION_TEMPORALITY_CUMULATIVE"
                    }
                }]
            }]
        }]
    }'
    
    local response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$metrics_data" \
        "$INGESTION_SERVICE_URL/v1/metrics")
    
    local http_code="${response: -3}"
    if [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✅ Metrics sent successfully${NC}"
    else
        echo -e "${RED}❌ Failed to send metrics (HTTP $http_code)${NC}"
        echo "Response: ${response%???}"
    fi
}

# Function to send sample logs
send_sample_logs() {
    echo -e "${YELLOW}📝 Sending sample logs...${NC}"
    
    # Sample logs data in OTLP format
    local logs_data='{
        "resourceLogs": [{
            "resource": {
                "attributes": [{
                    "key": "service.name",
                    "value": {"stringValue": "test-service"}
                }]
            },
            "scopeLogs": [{
                "scope": {
                    "name": "test-instrumentation",
                    "version": "1.0.0"
                },
                "logRecords": [{
                    "timeUnixNano": "'$(date +%s%N)'",
                    "severityNumber": 9,
                    "severityText": "INFO",
                    "body": {
                        "stringValue": "Test log message from telemorph-prime"
                    },
                    "attributes": [{
                        "key": "logger.name",
                        "value": {"stringValue": "test-logger"}
                    }, {
                        "key": "thread.name",
                        "value": {"stringValue": "main"}
                    }]
                }, {
                    "timeUnixNano": "'$(date +%s%N)'",
                    "severityNumber": 17,
                    "severityText": "ERROR",
                    "body": {
                        "stringValue": "Test error message for testing error handling"
                    },
                    "attributes": [{
                        "key": "logger.name",
                        "value": {"stringValue": "error-logger"}
                    }, {
                        "key": "error.type",
                        "value": {"stringValue": "TestError"}
                    }]
                }]
            }]
        }]
    }'
    
    local response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$logs_data" \
        "$INGESTION_SERVICE_URL/v1/logs")
    
    local http_code="${response: -3}"
    if [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✅ Logs sent successfully${NC}"
    else
        echo -e "${RED}❌ Failed to send logs (HTTP $http_code)${NC}"
        echo "Response: ${response%???}"
    fi
}

# Function to check Kafka topics
check_kafka_topics() {
    echo -e "${YELLOW}📦 Checking Kafka topics...${NC}"
    
    # Check if Kafka UI is accessible
    if curl -s -f "$KAFKA_UI_URL" > /dev/null; then
        echo -e "${GREEN}✅ Kafka UI is accessible at $KAFKA_UI_URL${NC}"
        echo "   You can view topics and messages in the web interface"
    else
        echo -e "${YELLOW}⚠️  Kafka UI not accessible, checking topics via CLI...${NC}"
        
        # Try to check topics using docker exec
        if command -v docker >/dev/null 2>&1; then
            echo "Checking Kafka topics..."
            docker exec telemorph-kafka kafka-topics --list --bootstrap-server localhost:9092 || echo "Could not list topics"
        fi
    fi
}

# Function to run continuous testing
run_continuous_test() {
    echo -e "${YELLOW}🔄 Running continuous test (sending data every 5 seconds)...${NC}"
    echo "Press Ctrl+C to stop"
    
    local count=1
    while true; do
        echo -e "\n${BLUE}--- Test Run #$count ---${NC}"
        send_sample_traces
        send_sample_metrics  
        send_sample_logs
        echo -e "${GREEN}✅ Batch $count completed${NC}"
        sleep 5
        ((count++))
    done
}

# Main execution
main() {
    case "${1:-all}" in
        "health")
            check_service
            ;;
        "traces")
            check_service
            send_sample_traces
            ;;
        "metrics")
            check_service
            send_sample_metrics
            ;;
        "logs")
            check_service
            send_sample_logs
            ;;
        "kafka")
            check_kafka_topics
            ;;
        "continuous")
            check_service
            run_continuous_test
            ;;
        "all"|*)
            check_service
            send_sample_traces
            send_sample_metrics
            send_sample_logs
            check_kafka_topics
            echo -e "\n${GREEN}🎉 All tests completed!${NC}"
            echo -e "${BLUE}Next steps:${NC}"
            echo "1. Check Kafka UI at $KAFKA_UI_URL to see the data in topics"
            echo "2. Run 'docker-compose logs ingestion-service' to see service logs"
            echo "3. Run './test-telemetry.sh continuous' for continuous testing"
            ;;
    esac
}

# Run main function with all arguments
main "$@"

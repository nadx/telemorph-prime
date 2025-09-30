#!/bin/bash

# Telemorph-Prime System Monitoring Script
# This script monitors all components of the ingestion pipeline

set -e

# Configuration
INGESTION_SERVICE_URL="http://localhost:4318"
HEALTH_URL="http://localhost:8080"
KAFKA_UI_URL="http://localhost:8081"
KAFKA_BROKER="localhost:9092"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print section headers
print_section() {
    echo -e "\n${BLUE}================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}================================================${NC}"
}

# Function to check service health
check_ingestion_service() {
    print_section "🏥 INGESTION SERVICE HEALTH"
    
    # Check health endpoint
    if curl -s -f "$HEALTH_URL/health" > /dev/null; then
        echo -e "${GREEN}✅ Health endpoint: OK${NC}"
    else
        echo -e "${RED}❌ Health endpoint: FAILED${NC}"
        return 1
    fi
    
    # Check readiness endpoint
    if curl -s -f "$HEALTH_URL/ready" > /dev/null; then
        echo -e "${GREEN}✅ Readiness endpoint: OK${NC}"
    else
        echo -e "${RED}❌ Readiness endpoint: FAILED${NC}"
    fi
    
    # Check metrics endpoint
    if curl -s -f "$HEALTH_URL/metrics" > /dev/null; then
        echo -e "${GREEN}✅ Metrics endpoint: OK${NC}"
    else
        echo -e "${RED}❌ Metrics endpoint: FAILED${NC}"
    fi
    
    # Get detailed health info
    echo -e "\n${CYAN}📊 Service Details:${NC}"
    curl -s "$HEALTH_URL/health" | jq '.' 2>/dev/null || echo "Could not parse JSON response"
}

# Function to check OTLP endpoints
check_otlp_endpoints() {
    print_section "📡 OTLP ENDPOINTS"
    
    # Test traces endpoint
    echo -e "${YELLOW}Testing traces endpoint...${NC}"
    local trace_response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d '{"test": "data"}' \
        "$INGESTION_SERVICE_URL/v1/traces" 2>/dev/null)
    
    local trace_code="${trace_response: -3}"
    if [ "$trace_code" = "200" ]; then
        echo -e "${GREEN}✅ Traces endpoint: OK${NC}"
    else
        echo -e "${RED}❌ Traces endpoint: FAILED (HTTP $trace_code)${NC}"
    fi
    
    # Test metrics endpoint
    echo -e "${YELLOW}Testing metrics endpoint...${NC}"
    local metrics_response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d '{"test": "data"}' \
        "$INGESTION_SERVICE_URL/v1/metrics" 2>/dev/null)
    
    local metrics_code="${metrics_response: -3}"
    if [ "$metrics_code" = "200" ]; then
        echo -e "${GREEN}✅ Metrics endpoint: OK${NC}"
    else
        echo -e "${RED}❌ Metrics endpoint: FAILED (HTTP $metrics_code)${NC}"
    fi
    
    # Test logs endpoint
    echo -e "${YELLOW}Testing logs endpoint...${NC}"
    local logs_response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d '{"test": "data"}' \
        "$INGESTION_SERVICE_URL/v1/logs" 2>/dev/null)
    
    local logs_code="${logs_response: -3}"
    if [ "$logs_code" = "200" ]; then
        echo -e "${GREEN}✅ Logs endpoint: OK${NC}"
    else
        echo -e "${RED}❌ Logs endpoint: FAILED (HTTP $logs_code)${NC}"
    fi
}

# Function to check Kafka connectivity
check_kafka() {
    print_section "📦 KAFKA CONNECTIVITY"
    
    # Check if Kafka is running
    if command -v docker >/dev/null 2>&1; then
        if docker ps | grep -q telemorph-kafka; then
            echo -e "${GREEN}✅ Kafka container: RUNNING${NC}"
        else
            echo -e "${RED}❌ Kafka container: NOT RUNNING${NC}"
            return 1
        fi
        
        # Check Kafka topics
        echo -e "\n${CYAN}📋 Kafka Topics:${NC}"
        docker exec telemorph-kafka kafka-topics --list --bootstrap-server localhost:9092 2>/dev/null || echo "Could not list topics"
        
        # Check topic details
        echo -e "\n${CYAN}📊 Topic Details:${NC}"
        for topic in otel.traces otel.metrics otel.logs; do
            echo -e "\n${YELLOW}Topic: $topic${NC}"
            docker exec telemorph-kafka kafka-topics --describe --topic "$topic" --bootstrap-server localhost:9092 2>/dev/null || echo "Topic not found or error"
        done
    else
        echo -e "${YELLOW}⚠️  Docker not available, skipping Kafka checks${NC}"
    fi
    
    # Check Kafka UI
    if curl -s -f "$KAFKA_UI_URL" > /dev/null; then
        echo -e "\n${GREEN}✅ Kafka UI: ACCESSIBLE at $KAFKA_UI_URL${NC}"
    else
        echo -e "\n${YELLOW}⚠️  Kafka UI: NOT ACCESSIBLE${NC}"
    fi
}

# Function to check Docker containers
check_containers() {
    print_section "🐳 DOCKER CONTAINERS"
    
    if command -v docker >/dev/null 2>&1; then
        echo -e "${CYAN}Container Status:${NC}"
        docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep telemorph || echo "No telemorph containers found"
        
        echo -e "\n${CYAN}Container Logs (last 10 lines):${NC}"
        for container in telemorph-ingestion telemorph-kafka telemorph-zookeeper; do
            if docker ps | grep -q "$container"; then
                echo -e "\n${YELLOW}$container:${NC}"
                docker logs --tail 10 "$container" 2>/dev/null || echo "Could not get logs"
            fi
        done
    else
        echo -e "${YELLOW}⚠️  Docker not available${NC}"
    fi
}

# Function to check system resources
check_resources() {
    print_section "💻 SYSTEM RESOURCES"
    
    echo -e "${CYAN}Memory Usage:${NC}"
    free -h 2>/dev/null || echo "Could not get memory info"
    
    echo -e "\n${CYAN}Disk Usage:${NC}"
    df -h 2>/dev/null || echo "Could not get disk info"
    
    echo -e "\n${CYAN}Docker Resource Usage:${NC}"
    if command -v docker >/dev/null 2>&1; then
        docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}" 2>/dev/null || echo "Could not get Docker stats"
    fi
}

# Function to run integration test
run_integration_test() {
    print_section "🧪 INTEGRATION TEST"
    
    echo -e "${YELLOW}Running quick integration test...${NC}"
    
    # Send test data
    local test_data='{
        "resourceSpans": [{
            "resource": {
                "attributes": [{
                    "key": "service.name",
                    "value": {"stringValue": "monitor-test"}
                }]
            },
            "scopeSpans": [{
                "scope": {"name": "monitor-test"},
                "spans": [{
                    "traceId": "monitor123456789012345678901234567890",
                    "spanId": "monitor1234567890123456",
                    "name": "monitor-test-operation",
                    "kind": "SPAN_KIND_SERVER",
                    "startTimeUnixNano": "'$(date +%s%N)'",
                    "endTimeUnixNano": "'$(($(date +%s%N) + 100000000))'",
                    "status": {"code": "STATUS_CODE_OK"}
                }]
            }]
        }]
    }'
    
    local response=$(curl -s -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$test_data" \
        "$INGESTION_SERVICE_URL/v1/traces" 2>/dev/null)
    
    local http_code="${response: -3}"
    if [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✅ Integration test: PASSED${NC}"
        echo -e "${CYAN}   Test data sent successfully to ingestion service${NC}"
    else
        echo -e "${RED}❌ Integration test: FAILED (HTTP $http_code)${NC}"
    fi
}

# Function to show monitoring dashboard
show_dashboard() {
    print_section "📊 MONITORING DASHBOARD"
    
    echo -e "${PURPLE}Telemorph-Prime Ingestion Service Status${NC}"
    echo -e "${PURPLE}==========================================${NC}"
    
    # Service status
    if curl -s -f "$HEALTH_URL/health" > /dev/null; then
        echo -e "🏥 Service: ${GREEN}HEALTHY${NC}"
    else
        echo -e "🏥 Service: ${RED}UNHEALTHY${NC}"
    fi
    
    # Kafka status
    if command -v docker >/dev/null 2>&1 && docker ps | grep -q telemorph-kafka; then
        echo -e "📦 Kafka: ${GREEN}RUNNING${NC}"
    else
        echo -e "📦 Kafka: ${RED}NOT RUNNING${NC}"
    fi
    
    # Endpoints
    echo -e "\n${CYAN}Available Endpoints:${NC}"
    echo -e "  • Health: $HEALTH_URL/health"
    echo -e "  • Metrics: $HEALTH_URL/metrics"
    echo -e "  • OTLP HTTP: $INGESTION_SERVICE_URL/v1/{traces,metrics,logs}"
    echo -e "  • Kafka UI: $KAFKA_UI_URL"
    
    echo -e "\n${CYAN}Quick Commands:${NC}"
    echo -e "  • Test data: ./test-telemetry.sh"
    echo -e "  • Check Kafka: python3 kafka-consumer-test.py"
    echo -e "  • View logs: docker-compose logs -f ingestion-service"
}

# Main execution
main() {
    case "${1:-all}" in
        "health")
            check_ingestion_service
            ;;
        "otlp")
            check_otlp_endpoints
            ;;
        "kafka")
            check_kafka
            ;;
        "containers")
            check_containers
            ;;
        "resources")
            check_resources
            ;;
        "test")
            run_integration_test
            ;;
        "dashboard")
            show_dashboard
            ;;
        "all"|*)
            check_ingestion_service
            check_otlp_endpoints
            check_kafka
            check_containers
            run_integration_test
            show_dashboard
            ;;
    esac
}

# Run main function with all arguments
main "$@"

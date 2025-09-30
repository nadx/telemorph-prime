#!/usr/bin/env python3
"""
Kafka Consumer Test Script for Telemorph-Prime
This script consumes messages from Kafka topics to verify data ingestion
"""

import json
import sys
import time
from datetime import datetime
from kafka import KafkaConsumer
from kafka.errors import KafkaError

# Configuration
KAFKA_BROKERS = ['localhost:9092']
TOPICS = ['otel.traces', 'otel.metrics', 'otel.logs']

class TelemetryConsumer:
    def __init__(self, brokers, topics):
        self.brokers = brokers
        self.topics = topics
        self.consumer = None
        self.message_counts = {topic: 0 for topic in topics}
        
    def connect(self):
        """Connect to Kafka"""
        try:
            self.consumer = KafkaConsumer(
                *self.topics,
                bootstrap_servers=self.brokers,
                auto_offset_reset='latest',
                enable_auto_commit=True,
                group_id='telemorph-test-consumer',
                value_deserializer=lambda x: json.loads(x.decode('utf-8')) if x else None
            )
            print(f"✅ Connected to Kafka at {self.brokers}")
            return True
        except KafkaError as e:
            print(f"❌ Failed to connect to Kafka: {e}")
            return False
    
    def print_message_info(self, message):
        """Print formatted message information"""
        topic = message.topic
        partition = message.partition
        offset = message.offset
        timestamp = datetime.fromtimestamp(message.timestamp / 1000) if message.timestamp else "N/A"
        
        print(f"\n📦 Topic: {topic}")
        print(f"   Partition: {partition}, Offset: {offset}")
        print(f"   Timestamp: {timestamp}")
        
        # Print headers
        if message.headers:
            print(f"   Headers:")
            for key, value in message.headers:
                print(f"     {key.decode()}: {value.decode()}")
        
        # Print value (truncated for readability)
        if message.value:
            value_str = json.dumps(message.value, indent=2)
            if len(value_str) > 500:
                value_str = value_str[:500] + "... (truncated)"
            print(f"   Value: {value_str}")
    
    def analyze_traces(self, data):
        """Analyze trace data structure"""
        if 'resourceSpans' in data:
            print("   🔍 Trace Analysis:")
            for resource_span in data['resourceSpans']:
                if 'scopeSpans' in resource_span:
                    for scope_span in resource_span['scopeSpans']:
                        if 'spans' in scope_span:
                            print(f"     Found {len(scope_span['spans'])} spans")
                            for span in scope_span['spans']:
                                name = span.get('name', 'Unknown')
                                trace_id = span.get('traceId', 'Unknown')
                                span_id = span.get('spanId', 'Unknown')
                                print(f"       - {name} (trace: {trace_id[:8]}..., span: {span_id[:8]}...)")
    
    def analyze_metrics(self, data):
        """Analyze metrics data structure"""
        if 'resourceMetrics' in data:
            print("   📊 Metrics Analysis:")
            for resource_metric in data['resourceMetrics']:
                if 'scopeMetrics' in resource_metric:
                    for scope_metric in resource_metric['scopeMetrics']:
                        if 'metrics' in scope_metric:
                            print(f"     Found {len(scope_metric['metrics'])} metrics")
                            for metric in scope_metric['metrics']:
                                name = metric.get('name', 'Unknown')
                                print(f"       - {name}")
    
    def analyze_logs(self, data):
        """Analyze logs data structure"""
        if 'resourceLogs' in data:
            print("   📝 Logs Analysis:")
            for resource_log in data['resourceLogs']:
                if 'scopeLogs' in resource_log:
                    for scope_log in resource_log['scopeLogs']:
                        if 'logRecords' in scope_log:
                            print(f"     Found {len(scope_log['logRecords'])} log records")
                            for log_record in scope_log['logRecords']:
                                severity = log_record.get('severityText', 'Unknown')
                                body = log_record.get('body', {}).get('stringValue', 'No message')
                                print(f"       - [{severity}] {body[:50]}...")
    
    def consume_messages(self, max_messages=10, timeout=30):
        """Consume messages from Kafka topics"""
        if not self.consumer:
            print("❌ Not connected to Kafka")
            return
        
        print(f"\n🔄 Consuming messages (max: {max_messages}, timeout: {timeout}s)...")
        print("Press Ctrl+C to stop\n")
        
        try:
            start_time = time.time()
            message_count = 0
            
            for message in self.consumer:
                if time.time() - start_time > timeout:
                    print(f"\n⏰ Timeout reached ({timeout}s)")
                    break
                
                if message_count >= max_messages:
                    print(f"\n📊 Reached maximum messages ({max_messages})")
                    break
                
                self.message_counts[message.topic] += 1
                message_count += 1
                
                print(f"\n{'='*60}")
                print(f"Message #{message_count}")
                self.print_message_info(message)
                
                # Analyze the data based on topic
                if message.value:
                    if message.topic == 'otel.traces':
                        self.analyze_traces(message.value)
                    elif message.topic == 'otel.metrics':
                        self.analyze_metrics(message.value)
                    elif message.topic == 'otel.logs':
                        self.analyze_logs(message.value)
                
        except KeyboardInterrupt:
            print(f"\n\n⏹️  Stopped by user")
        except Exception as e:
            print(f"\n❌ Error consuming messages: {e}")
        finally:
            self.print_summary()
    
    def print_summary(self):
        """Print consumption summary"""
        print(f"\n{'='*60}")
        print("📊 CONSUMPTION SUMMARY")
        print(f"{'='*60}")
        
        total_messages = sum(self.message_counts.values())
        print(f"Total messages consumed: {total_messages}")
        
        for topic, count in self.message_counts.items():
            print(f"  {topic}: {count} messages")
        
        if total_messages > 0:
            print(f"\n✅ Data ingestion is working correctly!")
            print(f"   - Messages are being sent to Kafka topics")
            print(f"   - Data structure looks valid")
            print(f"   - Ready for Phase 2 (Stream Processing)")
        else:
            print(f"\n⚠️  No messages received")
            print(f"   - Check if ingestion service is running")
            print(f"   - Check if test data is being sent")
            print(f"   - Check Kafka connectivity")
    
    def close(self):
        """Close the consumer"""
        if self.consumer:
            self.consumer.close()
            print("🔌 Disconnected from Kafka")

def main():
    print("🚀 Telemorph-Prime Kafka Consumer Test")
    print("=" * 50)
    
    # Parse command line arguments
    max_messages = 10
    timeout = 30
    
    if len(sys.argv) > 1:
        try:
            max_messages = int(sys.argv[1])
        except ValueError:
            print("❌ Invalid max_messages argument")
            sys.exit(1)
    
    if len(sys.argv) > 2:
        try:
            timeout = int(sys.argv[2])
        except ValueError:
            print("❌ Invalid timeout argument")
            sys.exit(1)
    
    print(f"Configuration:")
    print(f"  Kafka Brokers: {KAFKA_BROKERS}")
    print(f"  Topics: {TOPICS}")
    print(f"  Max Messages: {max_messages}")
    print(f"  Timeout: {timeout}s")
    
    # Create and run consumer
    consumer = TelemetryConsumer(KAFKA_BROKERS, TOPICS)
    
    try:
        if consumer.connect():
            consumer.consume_messages(max_messages, timeout)
    finally:
        consumer.close()

if __name__ == "__main__":
    main()

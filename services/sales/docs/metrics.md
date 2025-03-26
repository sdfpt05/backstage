# Sales Service Metrics Collection

## Overview

The Sales Service Metrics Collection system provides real-time monitoring and performance metrics for the sales service. It collects, aggregates, and exposes operational and business metrics with minimal performance impact.

## Key Components

- **Metrics Collector**: Central collection point for all metrics (`metrics.Collector`)
- **Prometheus Integration**: Exposes metrics in Prometheus format via HTTP endpoint
- **Service Instrumentation**: Hooks in critical code paths to collect metrics
- **WebSocket Server**: Provides real-time metrics updates to monitoring UI

## Metrics Types

The system collects four primary types of metrics:

1. **Counter Metrics**: Cumulative values that only increase
   - Total dispense sessions created
   - Total sales created
   - Total error count
   - Total sales amount

2. **Gauge Metrics**: Values that can increase or decrease
   - Active device count
   - Memory usage
   - Connection count

3. **Histogram Metrics**: Distribution of values
   - Processing latency
   - Request duration

4. **Business Metrics**: Derived metrics for business analysis
   - Sales conversion rate
   - Revenue by device

## Quick Start

### Recording Metrics

Add metrics collection to any service method:

```go
// Import the collector
import "sales_service/internal/metrics"

// In your service method
func (s *Service) CreateDispenseSession(...) {
    // Get the collector singleton
    collector := metrics.GetCollector()
    
    // Record metrics
    collector.DispenseSessionCreated(deviceMCU)
    
    // Record with a value
    collector.SaleCreated(amountKSH)
    
    // Record timing
    startTime := time.Now()
    // ... operation ...
    collector.DispenseSessionProcessed(time.Since(startTime).Seconds())
}
```

### Available Metrics Methods

- `DispenseSessionCreated(deviceMCU string)`: Records a new dispense session and updates active devices
- `DispenseSessionProcessed(latencySeconds float64)`: Records a processed session and its latency
- `SaleCreated(amountKSH int32)`: Records a new sale and updates sales amount total
- `RecordError(errorType string)`: Records an error occurrence
- `RecordAPICall(path string, duration float64)`: Records API call duration

## Accessing Metrics

### Prometheus Endpoint

Metrics are available in Prometheus format at the `/metrics` endpoint:

```
curl http://localhost:8092/metrics
```

Sample output:
```
# HELP sales_service_dispense_sessions_created_total The total number of dispense sessions created
# TYPE sales_service_dispense_sessions_created_total counter
sales_service_dispense_sessions_created_total 1423

# HELP sales_service_processing_latency_seconds The processing latency in seconds
# TYPE sales_service_processing_latency_seconds histogram
sales_service_processing_latency_seconds_bucket{le="0.01"} 756
sales_service_processing_latency_seconds_bucket{le="0.1"} 1211
...
```

### Monitoring Dashboard

The React monitoring dashboard connects to both the metrics endpoint and WebSocket to display:
- Real-time metrics visualization
- Historical trends
- Active device tracking
- Error monitoring

## Best Practices

1. **Performance Considerations**
   - Use asynchronous recording for non-critical metrics
   - Keep critical path instrumentation minimal

2. **Naming Conventions**
   - Use consistent naming: `sales_service_[entity]_[metric]_[unit]`
   - Example: `sales_service_processing_latency_seconds`

3. **Metric Selection**
   - Focus on actionable metrics that provide operational insights
   - Limit high-cardinality metrics (like per-device metrics) to avoid overhead

4. **Extending Metrics Collection**
   - Add new metrics to the Collector struct
   - Initialize in newCollector()
   - Add recording methods with appropriate documentation

## Configuration

The metrics system is configured through environment variables:

- `METRICS_ENABLED`: Enable/disable metrics collection (default: true)
- `METRICS_RETENTION_DAYS`: Days to retain metrics data (default: 15)
- `METRICS_SAMPLE_RATE`: Sampling rate for high-volume metrics (default: 1.0 = 100%)

## Troubleshooting

Common issues:

1. **High Memory Usage**
   - Check for high-cardinality metrics (too many labels)
   - Reduce retention period

2. **Missing Metrics**
   - Verify collection is enabled
   - Check instrumentation points in services

3. **Prometheus Connection Issues**
   - Verify `/metrics` endpoint is accessible
   - Check Prometheus scrape configuration
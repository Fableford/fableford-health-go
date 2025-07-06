# fableford-health-go

[![Go Reference](https://pkg.go.dev/badge/github.com/fableford/fableford-health-go.svg)](https://pkg.go.dev/github.com/fableford/fableford-health-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/fableford/fableford-health-go)](https://goreportcard.com/report/github.com/fableford/fableford-health-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A production-ready Go package providing standardized health check and monitoring endpoints for microservices. This package implements the foundation endpoints that should be consistent across all services in a distributed system.

## Features

- **Health Checks**: Basic health status endpoint for service monitoring
- **Liveness Probe**: Kubernetes-compatible liveness probe for container orchestration
- **Readiness Probe**: Readiness checks with dependency validation
- **Service Status**: Detailed service information including version, uptime, and dependencies
- **Metrics Export**: Prometheus-compatible metrics endpoint
- **High Test Coverage**: 92.5% test coverage with comprehensive table-driven tests
- **Production Ready**: Context support, proper error handling, and concurrent request handling

## Installation

```bash
go get github.com/fableford/fableford-health-go
```

## Quick Start

### Using the Client

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    health "github.com/fableford/fableford-health-go"
)

func main() {
    // Create a new client
    client, err := health.NewClient("http://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    
    // Check health status
    ctx := context.Background()
    healthResp, err := client.GetHealth(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Health Status: %s\n", healthResp.Status)
}
```

### Using the Server

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    
    "github.com/go-chi/chi/v5"
    health "github.com/fableford/fableford-health-go"
)

func main() {
    // Create a base server with your service details
    server := health.NewBaseServer("my-service", "0.1.0", "production")
    
    // Optionally set hostname
    hostname, _ := os.Hostname()
    server.Hostname = hostname
    
    // Add dependency checks
    server.CheckFunc = func(ctx context.Context) map[string]string {
        checks := make(map[string]string)
        
        // Add your dependency checks here
        checks["database"] = checkDatabase() 
        checks["cache"] = checkCache()
        
        return checks
    }
    
    // Create HTTP handler
    handler := health.NewHTTPHandler(server)
    
    // Register routes
    r := chi.NewRouter()
    handler.RegisterRoutes(r)
    
    // Start server
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", r))
}

func checkDatabase() string {
    // Implement your database check
    return "connected"
}

func checkCache() string {
    // Implement your cache check
    return "available"
}
```

## API Endpoints

### `GET /health`
Basic health status check.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-06T15:04:05Z"
}
```

### `GET /health/live`
Kubernetes liveness probe endpoint.

**Response:**
```json
{
  "alive": true,
  "timestamp": "2024-01-06T15:04:05Z"
}
```

### `GET /health/ready`
Readiness probe with dependency checks.

**Response:**
```json
{
  "ready": true,
  "timestamp": "2024-01-06T15:04:05Z",
  "checks": {
    "database": "connected",
    "cache": "available",
    "external_api": "reachable"
  }
}
```

### `GET /status`
Detailed service information.

**Response:**
```json
{
  "service_name": "my-service",
  "version": "0.1.0",
  "git_commit": "abc123def456",
  "build_time": "2024-01-06T10:00:00Z",
  "start_time": "2024-01-06T14:00:00Z",
  "uptime_seconds": 3665,
  "environment": "production",
  "hostname": "service-pod-xyz",
  "dependencies": [
    {
      "name": "postgres",
      "status": "healthy",
      "version": "14.5"
    },
    {
      "name": "redis",
      "status": "healthy",
      "version": "7.0"
    }
  ]
}
```

### `GET /metrics`
Prometheus-compatible metrics endpoint.

**Response:**
```
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 1234
# HELP http_request_duration_seconds HTTP request latency
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.005"} 24054
```

## Advanced Usage

### Custom Client Configuration

```go
// With custom timeout
client, err := health.NewClient("http://localhost:8080",
    health.WithTimeout(5 * time.Second),
)

// With custom HTTP client
httpClient := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns: 100,
        MaxIdleConnsPerHost: 10,
    },
}
client, err := health.NewClient("http://localhost:8080",
    health.WithHTTPClient(httpClient),
)
```

### Custom Server Implementation

```go
type MyServer struct {
    db *sql.DB
    cache *redis.Client
}

func (s *MyServer) GetHealth(ctx context.Context) (*health.HealthResponse, error) {
    // Custom health check logic
    if err := s.db.PingContext(ctx); err != nil {
        return &health.HealthResponse{
            Status: health.HealthStatusUnhealthy,
            Timestamp: time.Now(),
        }, nil
    }
    
    return &health.HealthResponse{
        Status: health.HealthStatusHealthy,
        Timestamp: time.Now(),
    }, nil
}

// Implement other interface methods...
```

### Custom Metrics

```go
server.MetricsFunc = func(ctx context.Context) (string, error) {
    // Return your custom Prometheus metrics
    return `# HELP my_custom_metric Custom metric description
# TYPE my_custom_metric counter
my_custom_metric{label="value"} 42
`, nil
}
```

## Testing

Run tests with coverage:

```bash
go test -v -race -cover ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./...
```

## OpenAPI Specification

The package includes a complete OpenAPI 3.0 specification in `openapi.yaml` that documents all endpoints, request/response schemas, and examples.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure:
- All tests pass
- Code coverage remains above 90%
- Code is properly formatted (`gofmt -w .`)
- No race conditions (`go test -race ./...`)

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built with [chi](https://github.com/go-chi/chi) router for HTTP handling
- Uses [testify](https://github.com/stretchr/testify) for enhanced testing capabilities
- Follows [Kubernetes best practices](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) for health checks

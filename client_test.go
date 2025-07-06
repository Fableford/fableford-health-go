package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		opts    []ClientOption
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid URL",
			baseURL: "http://localhost:8080",
			wantErr: false,
		},
		{
			name:    "valid URL with path",
			baseURL: "http://localhost:8080/api/v1",
			wantErr: false,
		},
		{
			name:    "invalid URL",
			baseURL: "://invalid",
			wantErr: true,
			errMsg:  "invalid base URL",
		},
		{
			name:    "with custom timeout",
			baseURL: "http://localhost:8080",
			opts:    []ClientOption{WithTimeout(5 * time.Second)},
			wantErr: false,
		},
		{
			name:    "with custom HTTP client",
			baseURL: "http://localhost:8080",
			opts:    []ClientOption{WithHTTPClient(&http.Client{Timeout: 10 * time.Second})},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.baseURL, tt.opts...)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestClient_GetHealth(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		responseStatus int
		responseBody   interface{}
		want           *HealthResponse
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "healthy response",
			responseStatus: http.StatusOK,
			responseBody: HealthResponse{
				Status:    HealthStatusHealthy,
				Timestamp: now,
			},
			want: &HealthResponse{
				Status:    HealthStatusHealthy,
				Timestamp: now,
			},
			wantErr: false,
		},
		{
			name:           "unhealthy response",
			responseStatus: http.StatusServiceUnavailable,
			responseBody: HealthResponse{
				Status:    HealthStatusUnhealthy,
				Timestamp: now,
			},
			want: &HealthResponse{
				Status:    HealthStatusUnhealthy,
				Timestamp: now,
			},
			wantErr: false,
		},
		{
			name:           "invalid status code",
			responseStatus: http.StatusBadRequest,
			responseBody:   "Bad Request",
			wantErr:        true,
			errMsg:         "unexpected status code 400",
		},
		{
			name:           "invalid JSON response",
			responseStatus: http.StatusOK,
			responseBody:   "invalid json",
			wantErr:        true,
			errMsg:         "decoding response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health", r.URL.Path)
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					switch v := tt.responseBody.(type) {
					case string:
						_, _ = w.Write([]byte(v))
					default:
						_ = json.NewEncoder(w).Encode(v)
					}
				}
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			got, err := client.GetHealth(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want.Status, got.Status)
				assert.WithinDuration(t, tt.want.Timestamp, got.Timestamp, time.Second)
			}
		})
	}
}

func TestClient_GetLiveness(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		responseStatus int
		responseBody   interface{}
		want           *LivenessResponse
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "alive response",
			responseStatus: http.StatusOK,
			responseBody: LivenessResponse{
				Alive:     true,
				Timestamp: now,
			},
			want: &LivenessResponse{
				Alive:     true,
				Timestamp: now,
			},
			wantErr: false,
		},
		{
			name:           "not alive response",
			responseStatus: http.StatusServiceUnavailable,
			responseBody: LivenessResponse{
				Alive:     false,
				Timestamp: now,
			},
			want: &LivenessResponse{
				Alive:     false,
				Timestamp: now,
			},
			wantErr: false,
		},
		{
			name:           "invalid status code",
			responseStatus: http.StatusInternalServerError,
			responseBody:   "Internal Server Error",
			wantErr:        true,
			errMsg:         "unexpected status code 500",
		},
		{
			name:           "context cancelled",
			responseStatus: http.StatusOK,
			responseBody: LivenessResponse{
				Alive:     true,
				Timestamp: now,
			},
			wantErr: true,
			errMsg:  "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health/live", r.URL.Path)
				assert.Equal(t, http.MethodGet, r.Method)

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					switch v := tt.responseBody.(type) {
					case string:
						_, _ = w.Write([]byte(v))
					default:
						_ = json.NewEncoder(w).Encode(v)
					}
				}
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			ctx := context.Background()
			if tt.name == "context cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			got, err := client.GetLiveness(ctx)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want.Alive, got.Alive)
				assert.WithinDuration(t, tt.want.Timestamp, got.Timestamp, time.Second)
			}
		})
	}
}

func TestClient_GetReadiness(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		responseStatus int
		responseBody   interface{}
		want           *ReadinessResponse
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "ready response",
			responseStatus: http.StatusOK,
			responseBody: ReadinessResponse{
				Ready:     true,
				Timestamp: now,
				Checks: map[string]string{
					"database": "connected",
					"cache":    "available",
				},
			},
			want: &ReadinessResponse{
				Ready:     true,
				Timestamp: now,
				Checks: map[string]string{
					"database": "connected",
					"cache":    "available",
				},
			},
			wantErr: false,
		},
		{
			name:           "not ready response",
			responseStatus: http.StatusServiceUnavailable,
			responseBody: ReadinessResponse{
				Ready:     false,
				Timestamp: now,
				Checks: map[string]string{
					"database": "connection failed",
					"cache":    "unavailable",
				},
			},
			want: &ReadinessResponse{
				Ready:     false,
				Timestamp: now,
				Checks: map[string]string{
					"database": "connection failed",
					"cache":    "unavailable",
				},
			},
			wantErr: false,
		},
		{
			name:           "empty checks",
			responseStatus: http.StatusOK,
			responseBody: ReadinessResponse{
				Ready:     true,
				Timestamp: now,
				Checks:    map[string]string{},
			},
			want: &ReadinessResponse{
				Ready:     true,
				Timestamp: now,
				Checks:    map[string]string{},
			},
			wantErr: false,
		},
		{
			name:           "network error",
			responseStatus: http.StatusOK,
			wantErr:        true,
			errMsg:         "executing request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "network error" {
				client, err := NewClient("http://invalid-host-that-does-not-exist:99999")
				require.NoError(t, err)

				_, err = client.GetReadiness(context.Background())
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/health/ready", r.URL.Path)
				assert.Equal(t, http.MethodGet, r.Method)

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					_ = json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			got, err := client.GetReadiness(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want.Ready, got.Ready)
				assert.WithinDuration(t, tt.want.Timestamp, got.Timestamp, time.Second)
				assert.Equal(t, tt.want.Checks, got.Checks)
			}
		})
	}
}

func TestClient_GetStatus(t *testing.T) {
	now := time.Now()
	buildTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name           string
		responseStatus int
		responseBody   interface{}
		want           *StatusResponse
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "full status response",
			responseStatus: http.StatusOK,
			responseBody: StatusResponse{
				ServiceName:   "test-service",
				Version:       "1.2.3",
				GitCommit:     "abc123",
				BuildTime:     &buildTime,
				StartTime:     now,
				UptimeSeconds: 3600,
				Environment:   "production",
				Hostname:      "test-host",
				Dependencies: []Dependency{
					{Name: "postgres", Status: "healthy", Version: "14.5"},
					{Name: "redis", Status: "healthy", Version: "7.0"},
				},
			},
			want: &StatusResponse{
				ServiceName:   "test-service",
				Version:       "1.2.3",
				GitCommit:     "abc123",
				BuildTime:     &buildTime,
				StartTime:     now,
				UptimeSeconds: 3600,
				Environment:   "production",
				Hostname:      "test-host",
				Dependencies: []Dependency{
					{Name: "postgres", Status: "healthy", Version: "14.5"},
					{Name: "redis", Status: "healthy", Version: "7.0"},
				},
			},
			wantErr: false,
		},
		{
			name:           "minimal status response",
			responseStatus: http.StatusOK,
			responseBody: StatusResponse{
				ServiceName:   "test-service",
				Version:       "1.0.0",
				StartTime:     now,
				UptimeSeconds: 0,
				Environment:   "development",
			},
			want: &StatusResponse{
				ServiceName:   "test-service",
				Version:       "1.0.0",
				StartTime:     now,
				UptimeSeconds: 0,
				Environment:   "development",
			},
			wantErr: false,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   "Internal Server Error",
			wantErr:        true,
			errMsg:         "unexpected status code 500",
		},
		{
			name:           "timeout error",
			responseStatus: http.StatusOK,
			wantErr:        true,
			errMsg:         "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/status", r.URL.Path)
				assert.Equal(t, http.MethodGet, r.Method)

				if tt.name == "timeout error" {
					time.Sleep(2 * time.Second)
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					switch v := tt.responseBody.(type) {
					case string:
						_, _ = w.Write([]byte(v))
					default:
						_ = json.NewEncoder(w).Encode(v)
					}
				}
			}))
			defer server.Close()

			client, err := NewClient(server.URL, WithTimeout(1*time.Second))
			require.NoError(t, err)

			got, err := client.GetStatus(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want.ServiceName, got.ServiceName)
				assert.Equal(t, tt.want.Version, got.Version)
				assert.Equal(t, tt.want.GitCommit, got.GitCommit)
				assert.Equal(t, tt.want.Environment, got.Environment)
				assert.Equal(t, tt.want.Hostname, got.Hostname)
				assert.Equal(t, tt.want.UptimeSeconds, got.UptimeSeconds)
				assert.Equal(t, tt.want.Dependencies, got.Dependencies)
				if tt.want.BuildTime != nil {
					assert.WithinDuration(t, *tt.want.BuildTime, *got.BuildTime, time.Second)
				}
				assert.WithinDuration(t, tt.want.StartTime, got.StartTime, time.Second)
			}
		})
	}
}

func TestClient_GetMetrics(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		contentType    string
		want           string
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "valid metrics response",
			responseStatus: http.StatusOK,
			responseBody: `# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 1234
`,
			contentType: "text/plain; version=0.0.4",
			want: `# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 1234
`,
			wantErr: false,
		},
		{
			name:           "empty metrics",
			responseStatus: http.StatusOK,
			responseBody:   "",
			contentType:    "text/plain",
			want:           "",
			wantErr:        false,
		},
		{
			name:           "large metrics response",
			responseStatus: http.StatusOK,
			responseBody:   generateLargeMetrics(),
			contentType:    "text/plain",
			want:           generateLargeMetrics(),
			wantErr:        false,
		},
		{
			name:           "not found",
			responseStatus: http.StatusNotFound,
			responseBody:   "Not Found",
			wantErr:        true,
			errMsg:         "unexpected status code 404",
		},
		{
			name:           "bad gateway",
			responseStatus: http.StatusBadGateway,
			responseBody:   "Bad Gateway",
			wantErr:        true,
			errMsg:         "unexpected status code 502",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/metrics", r.URL.Path)
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "text/plain", r.Header.Get("Accept"))

				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			require.NoError(t, err)

			got, err := client.GetMetrics(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestClient_ConcurrentRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)

		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(HealthResponse{
				Status:    HealthStatusHealthy,
				Timestamp: time.Now(),
			})
		case "/health/live":
			_ = json.NewEncoder(w).Encode(LivenessResponse{
				Alive:     true,
				Timestamp: time.Now(),
			})
		case "/health/ready":
			_ = json.NewEncoder(w).Encode(ReadinessResponse{
				Ready:     true,
				Timestamp: time.Now(),
				Checks:    map[string]string{"test": "ok"},
			})
		case "/status":
			_ = json.NewEncoder(w).Encode(StatusResponse{
				ServiceName:   "test",
				Version:       "1.0.0",
				StartTime:     time.Now(),
				UptimeSeconds: 100,
				Environment:   "test",
			})
		case "/metrics":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("# metrics"))
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	ctx := context.Background()
	errChan := make(chan error, 50)

	for i := 0; i < 10; i++ {
		go func() {
			_, err := client.GetHealth(ctx)
			errChan <- err
		}()
		go func() {
			_, err := client.GetLiveness(ctx)
			errChan <- err
		}()
		go func() {
			_, err := client.GetReadiness(ctx)
			errChan <- err
		}()
		go func() {
			_, err := client.GetStatus(ctx)
			errChan <- err
		}()
		go func() {
			_, err := client.GetMetrics(ctx)
			errChan <- err
		}()
	}

	for i := 0; i < 50; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}
}

func generateLargeMetrics() string {
	metrics := "# HELP test_metric Test metric\n# TYPE test_metric counter\n"
	for i := 0; i < 100; i++ {
		metrics += fmt.Sprintf("test_metric{label=\"value%d\"} %d\n", i, i*100)
	}
	return metrics
}

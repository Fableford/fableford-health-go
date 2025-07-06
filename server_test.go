package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockServer struct {
	healthFunc    func(ctx context.Context) (*HealthResponse, error)
	livenessFunc  func(ctx context.Context) (*LivenessResponse, error)
	readinessFunc func(ctx context.Context) (*ReadinessResponse, error)
	statusFunc    func(ctx context.Context) (*StatusResponse, error)
	metricsFunc   func(ctx context.Context) (string, error)
}

func (m *mockServer) GetHealth(ctx context.Context) (*HealthResponse, error) {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return &HealthResponse{Status: HealthStatusHealthy, Timestamp: time.Now()}, nil
}

func (m *mockServer) GetLiveness(ctx context.Context) (*LivenessResponse, error) {
	if m.livenessFunc != nil {
		return m.livenessFunc(ctx)
	}
	return &LivenessResponse{Alive: true, Timestamp: time.Now()}, nil
}

func (m *mockServer) GetReadiness(ctx context.Context) (*ReadinessResponse, error) {
	if m.readinessFunc != nil {
		return m.readinessFunc(ctx)
	}
	return &ReadinessResponse{Ready: true, Timestamp: time.Now(), Checks: map[string]string{}}, nil
}

func (m *mockServer) GetStatus(ctx context.Context) (*StatusResponse, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return &StatusResponse{
		ServiceName:   "test",
		Version:       "1.0.0",
		StartTime:     time.Now(),
		UptimeSeconds: 100,
		Environment:   "test",
	}, nil
}

func (m *mockServer) GetMetrics(ctx context.Context) (string, error) {
	if m.metricsFunc != nil {
		return m.metricsFunc(ctx)
	}
	return "# metrics", nil
}

func TestHTTPHandler_handleGetHealth(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		serverFunc   func(ctx context.Context) (*HealthResponse, error)
		wantStatus   int
		wantResponse *HealthResponse
		wantError    bool
	}{
		{
			name: "healthy status",
			serverFunc: func(ctx context.Context) (*HealthResponse, error) {
				return &HealthResponse{
					Status:    HealthStatusHealthy,
					Timestamp: now,
				}, nil
			},
			wantStatus: http.StatusOK,
			wantResponse: &HealthResponse{
				Status:    HealthStatusHealthy,
				Timestamp: now,
			},
		},
		{
			name: "unhealthy status",
			serverFunc: func(ctx context.Context) (*HealthResponse, error) {
				return &HealthResponse{
					Status:    HealthStatusUnhealthy,
					Timestamp: now,
				}, nil
			},
			wantStatus: http.StatusServiceUnavailable,
			wantResponse: &HealthResponse{
				Status:    HealthStatusUnhealthy,
				Timestamp: now,
			},
		},
		{
			name: "server error",
			serverFunc: func(ctx context.Context) (*HealthResponse, error) {
				return nil, errors.New("database connection failed")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockServer{healthFunc: tt.serverFunc}
			handler := NewHTTPHandler(mock)

			r := chi.NewRouter()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			if tt.wantError {
				var errResp map[string]string
				err := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, err)
				assert.Contains(t, errResp["error"], "database connection failed")
			} else {
				var resp HealthResponse
				err := json.NewDecoder(rec.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantResponse.Status, resp.Status)
				assert.WithinDuration(t, tt.wantResponse.Timestamp, resp.Timestamp, time.Second)
			}
		})
	}
}

func TestHTTPHandler_handleGetLiveness(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		serverFunc   func(ctx context.Context) (*LivenessResponse, error)
		wantStatus   int
		wantResponse *LivenessResponse
		wantError    bool
	}{
		{
			name: "alive",
			serverFunc: func(ctx context.Context) (*LivenessResponse, error) {
				return &LivenessResponse{
					Alive:     true,
					Timestamp: now,
				}, nil
			},
			wantStatus: http.StatusOK,
			wantResponse: &LivenessResponse{
				Alive:     true,
				Timestamp: now,
			},
		},
		{
			name: "not alive",
			serverFunc: func(ctx context.Context) (*LivenessResponse, error) {
				return &LivenessResponse{
					Alive:     false,
					Timestamp: now,
				}, nil
			},
			wantStatus: http.StatusServiceUnavailable,
			wantResponse: &LivenessResponse{
				Alive:     false,
				Timestamp: now,
			},
		},
		{
			name: "context cancelled",
			serverFunc: func(ctx context.Context) (*LivenessResponse, error) {
				return nil, context.Canceled
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockServer{livenessFunc: tt.serverFunc}
			handler := NewHTTPHandler(mock)

			r := chi.NewRouter()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantError {
				var errResp map[string]string
				err := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, err)
				assert.NotEmpty(t, errResp["error"])
			} else {
				var resp LivenessResponse
				err := json.NewDecoder(rec.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantResponse.Alive, resp.Alive)
				assert.WithinDuration(t, tt.wantResponse.Timestamp, resp.Timestamp, time.Second)
			}
		})
	}
}

func TestHTTPHandler_handleGetReadiness(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		serverFunc   func(ctx context.Context) (*ReadinessResponse, error)
		wantStatus   int
		wantResponse *ReadinessResponse
		wantError    bool
	}{
		{
			name: "ready with checks",
			serverFunc: func(ctx context.Context) (*ReadinessResponse, error) {
				return &ReadinessResponse{
					Ready:     true,
					Timestamp: now,
					Checks: map[string]string{
						"database": "connected",
						"cache":    "available",
					},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantResponse: &ReadinessResponse{
				Ready:     true,
				Timestamp: now,
				Checks: map[string]string{
					"database": "connected",
					"cache":    "available",
				},
			},
		},
		{
			name: "not ready",
			serverFunc: func(ctx context.Context) (*ReadinessResponse, error) {
				return &ReadinessResponse{
					Ready:     false,
					Timestamp: now,
					Checks: map[string]string{
						"database": "connection failed",
						"cache":    "timeout",
					},
				}, nil
			},
			wantStatus: http.StatusServiceUnavailable,
			wantResponse: &ReadinessResponse{
				Ready:     false,
				Timestamp: now,
				Checks: map[string]string{
					"database": "connection failed",
					"cache":    "timeout",
				},
			},
		},
		{
			name: "empty checks",
			serverFunc: func(ctx context.Context) (*ReadinessResponse, error) {
				return &ReadinessResponse{
					Ready:     true,
					Timestamp: now,
					Checks:    map[string]string{},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantResponse: &ReadinessResponse{
				Ready:     true,
				Timestamp: now,
				Checks:    map[string]string{},
			},
		},
		{
			name: "timeout error",
			serverFunc: func(ctx context.Context) (*ReadinessResponse, error) {
				return nil, context.DeadlineExceeded
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockServer{readinessFunc: tt.serverFunc}
			handler := NewHTTPHandler(mock)

			r := chi.NewRouter()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantError {
				var errResp map[string]string
				err := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, err)
				assert.NotEmpty(t, errResp["error"])
			} else {
				var resp ReadinessResponse
				err := json.NewDecoder(rec.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantResponse.Ready, resp.Ready)
				assert.WithinDuration(t, tt.wantResponse.Timestamp, resp.Timestamp, time.Second)
				assert.Equal(t, tt.wantResponse.Checks, resp.Checks)
			}
		})
	}
}

func TestHTTPHandler_handleGetStatus(t *testing.T) {
	now := time.Now()
	buildTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name         string
		serverFunc   func(ctx context.Context) (*StatusResponse, error)
		wantStatus   int
		wantResponse *StatusResponse
		wantError    bool
	}{
		{
			name: "full status",
			serverFunc: func(ctx context.Context) (*StatusResponse, error) {
				return &StatusResponse{
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
				}, nil
			},
			wantStatus: http.StatusOK,
			wantResponse: &StatusResponse{
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
		},
		{
			name: "minimal status",
			serverFunc: func(ctx context.Context) (*StatusResponse, error) {
				return &StatusResponse{
					ServiceName:   "test-service",
					Version:       "1.0.0",
					StartTime:     now,
					UptimeSeconds: 0,
					Environment:   "development",
				}, nil
			},
			wantStatus: http.StatusOK,
			wantResponse: &StatusResponse{
				ServiceName:   "test-service",
				Version:       "1.0.0",
				StartTime:     now,
				UptimeSeconds: 0,
				Environment:   "development",
			},
		},
		{
			name: "internal error",
			serverFunc: func(ctx context.Context) (*StatusResponse, error) {
				return nil, errors.New("failed to get hostname")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockServer{statusFunc: tt.serverFunc}
			handler := NewHTTPHandler(mock)

			r := chi.NewRouter()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, "/status", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantError {
				var errResp map[string]string
				err := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, err)
				assert.Contains(t, errResp["error"], "failed to get hostname")
			} else {
				var resp StatusResponse
				err := json.NewDecoder(rec.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantResponse.ServiceName, resp.ServiceName)
				assert.Equal(t, tt.wantResponse.Version, resp.Version)
				assert.Equal(t, tt.wantResponse.GitCommit, resp.GitCommit)
				assert.Equal(t, tt.wantResponse.Environment, resp.Environment)
				assert.Equal(t, tt.wantResponse.Hostname, resp.Hostname)
				assert.Equal(t, tt.wantResponse.UptimeSeconds, resp.UptimeSeconds)
				assert.Equal(t, tt.wantResponse.Dependencies, resp.Dependencies)
				if tt.wantResponse.BuildTime != nil {
					assert.WithinDuration(t, *tt.wantResponse.BuildTime, *resp.BuildTime, time.Second)
				}
				assert.WithinDuration(t, tt.wantResponse.StartTime, resp.StartTime, time.Second)
			}
		})
	}
}

func TestHTTPHandler_handleGetMetrics(t *testing.T) {
	tests := []struct {
		name         string
		serverFunc   func(ctx context.Context) (string, error)
		wantStatus   int
		wantResponse string
		wantType     string
		wantError    bool
	}{
		{
			name: "valid metrics",
			serverFunc: func(ctx context.Context) (string, error) {
				return `# HELP http_requests_total Total HTTP requests
# TYPE http_requests_total counter
http_requests_total 1234
`, nil
			},
			wantStatus: http.StatusOK,
			wantResponse: `# HELP http_requests_total Total HTTP requests
# TYPE http_requests_total counter
http_requests_total 1234
`,
			wantType: "text/plain; version=0.0.4",
		},
		{
			name: "empty metrics",
			serverFunc: func(ctx context.Context) (string, error) {
				return "", nil
			},
			wantStatus:   http.StatusOK,
			wantResponse: "",
			wantType:     "text/plain; version=0.0.4",
		},
		{
			name: "metrics error",
			serverFunc: func(ctx context.Context) (string, error) {
				return "", errors.New("prometheus registry error")
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockServer{metricsFunc: tt.serverFunc}
			handler := NewHTTPHandler(mock)

			r := chi.NewRouter()
			handler.RegisterRoutes(r)

			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantError {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
				var errResp map[string]string
				err := json.NewDecoder(rec.Body).Decode(&errResp)
				require.NoError(t, err)
				assert.Contains(t, errResp["error"], "prometheus registry error")
			} else {
				assert.Equal(t, tt.wantType, rec.Header().Get("Content-Type"))
				body, err := io.ReadAll(rec.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.wantResponse, string(body))
			}
		})
	}
}

func TestBaseServer_GetHealth(t *testing.T) {
	server := NewBaseServer("test-service", "1.0.0", "test")
	resp, err := server.GetHealth(context.Background())

	require.NoError(t, err)
	assert.Equal(t, HealthStatusHealthy, resp.Status)
	assert.WithinDuration(t, time.Now(), resp.Timestamp, time.Second)
}

func TestBaseServer_GetLiveness(t *testing.T) {
	server := NewBaseServer("test-service", "1.0.0", "test")
	resp, err := server.GetLiveness(context.Background())

	require.NoError(t, err)
	assert.True(t, resp.Alive)
	assert.WithinDuration(t, time.Now(), resp.Timestamp, time.Second)
}

func TestBaseServer_GetReadiness(t *testing.T) {
	tests := []struct {
		name       string
		checkFunc  func(ctx context.Context) map[string]string
		wantReady  bool
		wantChecks map[string]string
	}{
		{
			name:       "no checks",
			checkFunc:  nil,
			wantReady:  true,
			wantChecks: map[string]string{},
		},
		{
			name: "all checks pass",
			checkFunc: func(ctx context.Context) map[string]string {
				return map[string]string{
					"database": "connected",
					"cache":    "available",
					"api":      "reachable",
				}
			},
			wantReady: true,
			wantChecks: map[string]string{
				"database": "connected",
				"cache":    "available",
				"api":      "reachable",
			},
		},
		{
			name: "some checks fail",
			checkFunc: func(ctx context.Context) map[string]string {
				return map[string]string{
					"database": "connected",
					"cache":    "connection failed",
					"api":      "timeout",
				}
			},
			wantReady: false,
			wantChecks: map[string]string{
				"database": "connected",
				"cache":    "connection failed",
				"api":      "timeout",
			},
		},
		{
			name: "all checks fail",
			checkFunc: func(ctx context.Context) map[string]string {
				return map[string]string{
					"database": "connection refused",
					"cache":    "unavailable",
				}
			},
			wantReady: false,
			wantChecks: map[string]string{
				"database": "connection refused",
				"cache":    "unavailable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewBaseServer("test-service", "1.0.0", "test")
			server.CheckFunc = tt.checkFunc

			resp, err := server.GetReadiness(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.wantReady, resp.Ready)
			assert.Equal(t, tt.wantChecks, resp.Checks)
			assert.WithinDuration(t, time.Now(), resp.Timestamp, time.Second)
		})
	}
}

func TestBaseServer_GetStatus(t *testing.T) {
	now := time.Now()
	buildTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name   string
		server *BaseServer
	}{
		{
			name: "minimal server",
			server: &BaseServer{
				ServiceName: "test-service",
				Version:     "1.0.0",
				StartTime:   now,
				Environment: "test",
			},
		},
		{
			name: "full server",
			server: &BaseServer{
				ServiceName: "test-service",
				Version:     "1.2.3",
				GitCommit:   "abc123def456",
				BuildTime:   &buildTime,
				StartTime:   now.Add(-1 * time.Hour),
				Environment: "production",
				Hostname:    "test-host-123",
				Dependencies: []Dependency{
					{Name: "postgres", Status: "healthy", Version: "14.5"},
					{Name: "redis", Status: "healthy", Version: "7.0"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.server.GetStatus(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.server.ServiceName, resp.ServiceName)
			assert.Equal(t, tt.server.Version, resp.Version)
			assert.Equal(t, tt.server.GitCommit, resp.GitCommit)
			assert.Equal(t, tt.server.Environment, resp.Environment)
			assert.Equal(t, tt.server.Hostname, resp.Hostname)
			assert.Equal(t, tt.server.Dependencies, resp.Dependencies)

			if tt.server.BuildTime != nil {
				assert.WithinDuration(t, *tt.server.BuildTime, *resp.BuildTime, time.Second)
			}

			expectedUptime := time.Since(tt.server.StartTime).Seconds()
			assert.InDelta(t, expectedUptime, float64(resp.UptimeSeconds), 1.0)
		})
	}
}

func TestBaseServer_GetMetrics(t *testing.T) {
	tests := []struct {
		name        string
		metricsFunc func(ctx context.Context) (string, error)
		want        string
		wantErr     bool
	}{
		{
			name:        "default metrics",
			metricsFunc: nil,
			want: `# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 0
# HELP http_request_duration_seconds HTTP request latency
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.005"} 0
http_request_duration_seconds_bucket{le="0.01"} 0
http_request_duration_seconds_bucket{le="0.025"} 0
http_request_duration_seconds_sum 0
http_request_duration_seconds_count 0
`,
		},
		{
			name: "custom metrics",
			metricsFunc: func(ctx context.Context) (string, error) {
				return "# custom metrics\ncustom_metric 42\n", nil
			},
			want: "# custom metrics\ncustom_metric 42\n",
		},
		{
			name: "metrics error",
			metricsFunc: func(ctx context.Context) (string, error) {
				return "", errors.New("metrics collection failed")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewBaseServer("test-service", "1.0.0", "test")
			server.MetricsFunc = tt.metricsFunc

			got, err := server.GetMetrics(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "metrics collection failed")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestHTTPHandler_MethodNotAllowed(t *testing.T) {
	handler := NewHTTPHandler(&mockServer{})
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	endpoints := []string{"/health", "/health/live", "/health/ready", "/status", "/metrics"}
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, endpoint := range endpoints {
		for _, method := range methods {
			t.Run(endpoint+"_"+method, func(t *testing.T) {
				req := httptest.NewRequest(method, endpoint, nil)
				rec := httptest.NewRecorder()

				r.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			})
		}
	}
}

func TestHTTPHandler_ConcurrentRequests(t *testing.T) {
	handler := NewHTTPHandler(&mockServer{})
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	endpoints := []string{"/health", "/health/live", "/health/ready", "/status", "/metrics"}

	for i := 0; i < 10; i++ {
		for _, endpoint := range endpoints {
			go func(ep string) {
				req := httptest.NewRequest(http.MethodGet, ep, nil)
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusOK, rec.Code)
			}(endpoint)
		}
	}

	time.Sleep(100 * time.Millisecond)
}

func TestHTTPHandler_LargeResponse(t *testing.T) {
	largeMetrics := strings.Repeat("metric_name{label=\"value\"} 123\n", 10000)

	mock := &mockServer{
		metricsFunc: func(ctx context.Context) (string, error) {
			return largeMetrics, nil
		},
	}

	handler := NewHTTPHandler(mock)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Equal(t, largeMetrics, string(body))
}

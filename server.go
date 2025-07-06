package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Server interface {
	GetHealth(ctx context.Context) (*HealthResponse, error)
	GetLiveness(ctx context.Context) (*LivenessResponse, error)
	GetReadiness(ctx context.Context) (*ReadinessResponse, error)
	GetStatus(ctx context.Context) (*StatusResponse, error)
	GetMetrics(ctx context.Context) (string, error)
}

type HTTPHandler struct {
	server Server
}

func NewHTTPHandler(server Server) *HTTPHandler {
	return &HTTPHandler{
		server: server,
	}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Get("/health", h.handleGetHealth)
	r.Get("/health/live", h.handleGetLiveness)
	r.Get("/health/ready", h.handleGetReadiness)
	r.Get("/status", h.handleGetStatus)
	r.Get("/metrics", h.handleGetMetrics)
}

func (h *HTTPHandler) handleGetHealth(w http.ResponseWriter, r *http.Request) {
	resp, err := h.server.GetHealth(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err)
		return
	}

	status := http.StatusOK
	if resp.Status == HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	}

	h.writeJSON(w, status, resp)
}

func (h *HTTPHandler) handleGetLiveness(w http.ResponseWriter, r *http.Request) {
	resp, err := h.server.GetLiveness(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err)
		return
	}

	status := http.StatusOK
	if !resp.Alive {
		status = http.StatusServiceUnavailable
	}

	h.writeJSON(w, status, resp)
}

func (h *HTTPHandler) handleGetReadiness(w http.ResponseWriter, r *http.Request) {
	resp, err := h.server.GetReadiness(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err)
		return
	}

	status := http.StatusOK
	if !resp.Ready {
		status = http.StatusServiceUnavailable
	}

	h.writeJSON(w, status, resp)
}

func (h *HTTPHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	resp, err := h.server.GetStatus(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err)
		return
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *HTTPHandler) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.server.GetMetrics(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(metrics))
}

func (h *HTTPHandler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *HTTPHandler) writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}

type BaseServer struct {
	ServiceName  string
	Version      string
	GitCommit    string
	BuildTime    *time.Time
	StartTime    time.Time
	Environment  string
	Hostname     string
	CheckFunc    func(ctx context.Context) map[string]string
	MetricsFunc  func(ctx context.Context) (string, error)
	Dependencies []Dependency
}

func NewBaseServer(serviceName, version, environment string) *BaseServer {
	return &BaseServer{
		ServiceName: serviceName,
		Version:     version,
		StartTime:   time.Now(),
		Environment: environment,
	}
}

func (s *BaseServer) GetHealth(ctx context.Context) (*HealthResponse, error) {
	return &HealthResponse{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now(),
	}, nil
}

func (s *BaseServer) GetLiveness(ctx context.Context) (*LivenessResponse, error) {
	return &LivenessResponse{
		Alive:     true,
		Timestamp: time.Now(),
	}, nil
}

func (s *BaseServer) GetReadiness(ctx context.Context) (*ReadinessResponse, error) {
	checks := make(map[string]string)
	ready := true

	if s.CheckFunc != nil {
		checks = s.CheckFunc(ctx)
		for _, status := range checks {
			if status != "connected" && status != "available" && status != "reachable" && status != "healthy" {
				ready = false
				break
			}
		}
	}

	return &ReadinessResponse{
		Ready:     ready,
		Timestamp: time.Now(),
		Checks:    checks,
	}, nil
}

func (s *BaseServer) GetStatus(ctx context.Context) (*StatusResponse, error) {
	uptime := time.Since(s.StartTime).Seconds()

	return &StatusResponse{
		ServiceName:   s.ServiceName,
		Version:       s.Version,
		GitCommit:     s.GitCommit,
		BuildTime:     s.BuildTime,
		StartTime:     s.StartTime,
		UptimeSeconds: int64(uptime),
		Environment:   s.Environment,
		Hostname:      s.Hostname,
		Dependencies:  s.Dependencies,
	}, nil
}

func (s *BaseServer) GetMetrics(ctx context.Context) (string, error) {
	if s.MetricsFunc != nil {
		return s.MetricsFunc(ctx)
	}

	defaultMetrics := `# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 0
# HELP http_request_duration_seconds HTTP request latency
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.005"} 0
http_request_duration_seconds_bucket{le="0.01"} 0
http_request_duration_seconds_bucket{le="0.025"} 0
http_request_duration_seconds_sum 0
http_request_duration_seconds_count 0
`
	return defaultMetrics, nil
}

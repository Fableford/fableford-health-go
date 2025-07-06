package health

import (
	"time"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type HealthResponse struct {
	Status    HealthStatus `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
}

type LivenessResponse struct {
	Alive     bool      `json:"alive"`
	Timestamp time.Time `json:"timestamp"`
}

type ReadinessResponse struct {
	Ready     bool              `json:"ready"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

type StatusResponse struct {
	ServiceName   string       `json:"service_name"`
	Version       string       `json:"version"`
	GitCommit     string       `json:"git_commit,omitempty"`
	BuildTime     *time.Time   `json:"build_time,omitempty"`
	StartTime     time.Time    `json:"start_time"`
	UptimeSeconds int64        `json:"uptime_seconds"`
	Environment   string       `json:"environment"`
	Hostname      string       `json:"hostname,omitempty"`
	Dependencies  []Dependency `json:"dependencies,omitempty"`
}

type Dependency struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

package health

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client interface {
	GetHealth(ctx context.Context) (*HealthResponse, error)
	GetLiveness(ctx context.Context) (*LivenessResponse, error)
	GetReadiness(ctx context.Context) (*ReadinessResponse, error)
	GetStatus(ctx context.Context) (*StatusResponse, error)
	GetMetrics(ctx context.Context) (string, error)
}

type ClientOption func(*client)

type client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, opts ...ClientOption) (Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	c := &client{
		baseURL: u.String(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *client) {
		c.httpClient.Timeout = timeout
	}
}

func (c *client) doRequest(ctx context.Context, method, path string) (*http.Response, error) {
	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

func (c *client) GetHealth(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var healthResp HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &healthResp, nil
}

func (c *client) GetLiveness(ctx context.Context) (*LivenessResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health/live")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var livenessResp LivenessResponse
	if err := json.NewDecoder(resp.Body).Decode(&livenessResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &livenessResp, nil
}

func (c *client) GetReadiness(ctx context.Context) (*ReadinessResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health/ready")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var readinessResp ReadinessResponse
	if err := json.NewDecoder(resp.Body).Decode(&readinessResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &readinessResp, nil
}

func (c *client) GetStatus(ctx context.Context) (*StatusResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var statusResp StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &statusResp, nil
}

func (c *client) GetMetrics(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/metrics", nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	return string(body), nil
}

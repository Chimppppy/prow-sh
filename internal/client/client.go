package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prow-sh/prow/pkg/pcs"
)

// Client is a thin HTTP client for prowd (used by the prow CLI).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) baseURL() string {
	return strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
}

func (c *Client) newRequest(ctx context.Context, method, path string) (*http.Request, error) {
	u := c.baseURL() + path
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}
	return req, nil
}

// Health calls GET /health.
func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/health")
	if err != nil {
		return HealthResponse{}, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return HealthResponse{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return HealthResponse{}, fmt.Errorf("health: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out HealthResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return HealthResponse{}, fmt.Errorf("health: decode: %w", err)
	}
	return out, nil
}

// Version calls GET /version.
func (c *Client) Version(ctx context.Context) (VersionResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/version")
	if err != nil {
		return VersionResponse{}, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return VersionResponse{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return VersionResponse{}, fmt.Errorf("version: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out VersionResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return VersionResponse{}, fmt.Errorf("version: decode: %w", err)
	}
	return out, nil
}

// Events calls GET /v1/events.
func (c *Client) Events(ctx context.Context) ([]pcs.Event, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/v1/events")
	if err != nil {
		return nil, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("events: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out []pcs.Event
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("events: decode: %w", err)
	}
	return out, nil
}

// New returns a client with sane defaults.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

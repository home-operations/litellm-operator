// Package litellmclient is a focused typed client for the LiteLLM proxy admin
// API endpoints the operator uses in api mode (DB-backed model management).
package litellmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client talks to a LiteLLM proxy's admin API authenticated with the master key.
type Client struct {
	endpoint string
	key      string
	http     *http.Client
}

// New returns a client for the given admin API endpoint and master key.
func New(endpoint, masterKey string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), key: masterKey, http: hc}
}

// Model mirrors a model_list entry as the admin API accepts and returns it.
type Model struct {
	ModelName     string         `json:"model_name"`
	LiteLLMParams map[string]any `json:"litellm_params,omitempty"`
	ModelInfo     map[string]any `json:"model_info,omitempty"`
}

// ModelID returns the server-assigned id from model_info, if present.
func (m Model) ModelID() string {
	if m.ModelInfo == nil {
		return ""
	}
	id, _ := m.ModelInfo["id"].(string)
	return id
}

// ListModels returns the models currently registered on the proxy (GET /model/info).
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	var out struct {
		Data []Model `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/model/info", nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// CreateModel adds a model (POST /model/new).
func (c *Client) CreateModel(ctx context.Context, m Model) error {
	return c.do(ctx, http.MethodPost, "/model/new", m, nil)
}

// UpdateModel updates a model in place (POST /model/update); m.ModelInfo["id"] must be set.
func (c *Client) UpdateModel(ctx context.Context, m Model) error {
	return c.do(ctx, http.MethodPost, "/model/update", m, nil)
}

// DeleteModel removes a model by id (POST /model/delete).
func (c *Client) DeleteModel(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/model/delete", map[string]string{"id": id}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s body: %w", path, err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if out != nil {
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("decode %s response: %w", path, err)
		}
	}
	return nil
}

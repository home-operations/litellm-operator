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
	"net/url"
	"strings"
)

const teamIDJSONKey = "team_id"

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

// ObjectPermission contains the MCP toolset assignments managed on a virtual key.
type ObjectPermission struct {
	MCPToolsets []string `json:"mcp_toolsets"`
}

// VirtualKeyRequest describes a virtual key accepted by POST /key/generate.
type VirtualKeyRequest struct {
	KeyAlias            string            `json:"key_alias,omitempty"`
	Models              []string          `json:"models,omitempty"`
	ObjectPermission    *ObjectPermission `json:"object_permission,omitempty"`
	Aliases             map[string]string `json:"aliases,omitempty"`
	UserID              string            `json:"user_id,omitempty"`
	TeamID              string            `json:"team_id,omitempty"`
	Duration            string            `json:"duration,omitempty"`
	MaxBudget           *float64          `json:"max_budget,omitempty"`
	BudgetDuration      string            `json:"budget_duration,omitempty"`
	MaxParallelRequests *int64            `json:"max_parallel_requests,omitempty"`
	TPMLimit            *int64            `json:"tpm_limit,omitempty"`
	RPMLimit            *int64            `json:"rpm_limit,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

// VirtualKey is the sensitive portion of the response from POST /key/generate.
type VirtualKey struct {
	Key                 string            `json:"key,omitempty"`
	KeyAlias            string            `json:"key_alias,omitempty"`
	Models              []string          `json:"models,omitempty"`
	ObjectPermission    *ObjectPermission `json:"object_permission,omitempty"`
	Aliases             map[string]string `json:"aliases,omitempty"`
	UserID              string            `json:"user_id,omitempty"`
	TeamID              string            `json:"team_id,omitempty"`
	Duration            string            `json:"duration,omitempty"`
	Expires             *string           `json:"expires,omitempty"`
	MaxBudget           *float64          `json:"max_budget,omitempty"`
	BudgetDuration      string            `json:"budget_duration,omitempty"`
	MaxParallelRequests *int64            `json:"max_parallel_requests,omitempty"`
	TPMLimit            *int64            `json:"tpm_limit,omitempty"`
	RPMLimit            *int64            `json:"rpm_limit,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

// TeamMember identifies a LiteLLM team member and their role.
type TeamMember struct {
	UserID    string `json:"user_id,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
	Role      string `json:"role"`
}

// TeamObjectPermission contains the MCP server assignments managed on a team.
type TeamObjectPermission struct {
	MCPServers []string `json:"mcp_servers"`
}

// TeamRequest describes a team accepted by LiteLLM's team management endpoints.
type TeamRequest struct {
	TeamID           string                `json:"team_id"`
	TeamAlias        string                `json:"team_alias,omitempty"`
	Members          []TeamMember          `json:"members_with_roles,omitempty"`
	Models           []string              `json:"models"`
	ObjectPermission *TeamObjectPermission `json:"object_permission,omitempty"`
	MaxBudget        *float64              `json:"max_budget,omitempty"`
	BudgetDuration   string                `json:"budget_duration,omitempty"`
	TPMLimit         *int64                `json:"tpm_limit,omitempty"`
	RPMLimit         *int64                `json:"rpm_limit,omitempty"`
	Metadata         map[string]string     `json:"metadata,omitempty"`
	Blocked          *bool                 `json:"blocked,omitempty"`
}

// Team is the subset of a LiteLLM team response used by the operator.
type Team struct {
	TeamID           string                `json:"team_id"`
	Members          []TeamMember          `json:"members_with_roles"`
	ObjectPermission *TeamObjectPermission `json:"object_permission,omitempty"`
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

// GenerateVirtualKey creates a virtual key (POST /key/generate).
func (c *Client) GenerateVirtualKey(ctx context.Context, key VirtualKeyRequest) (VirtualKey, error) {
	if key.ObjectPermission != nil && key.ObjectPermission.MCPToolsets == nil {
		key.ObjectPermission = &ObjectPermission{MCPToolsets: []string{}}
	}
	var out VirtualKey
	if err := c.do(ctx, http.MethodPost, "/key/generate", key, &out); err != nil {
		return VirtualKey{}, err
	}
	return out, nil
}

// GetVirtualKey returns a virtual key's current settings (GET /key/info).
func (c *Client) GetVirtualKey(ctx context.Context, key string) (VirtualKey, error) {
	var out struct {
		Info *VirtualKey `json:"info"`
	}
	if err := c.do(ctx, http.MethodGet, "/key/info?key="+url.QueryEscape(key), nil, &out); err != nil {
		return VirtualKey{}, err
	}
	if out.Info == nil {
		return VirtualKey{}, fmt.Errorf("litellmclient: /key/info response has no info")
	}
	return *out.Info, nil
}

// UpdateVirtualKey updates a virtual key in place (POST /key/update).
// If updateDuration is false, expiry is left unchanged.
func (c *Client) UpdateVirtualKey(ctx context.Context, key string, request VirtualKeyRequest, updateDuration bool) error {
	if request.Models == nil {
		request.Models = []string{}
	}
	if request.Aliases == nil {
		request.Aliases = map[string]string{}
	}
	if request.Metadata == nil {
		request.Metadata = map[string]string{}
	}
	// LiteLLM treats omitted fields as unchanged.
	body := map[string]any{
		"key":                   key,
		"models":                request.Models,
		"aliases":               request.Aliases,
		"metadata":              request.Metadata,
		"max_budget":            request.MaxBudget,
		"max_parallel_requests": request.MaxParallelRequests,
		"tpm_limit":             request.TPMLimit,
		"rpm_limit":             request.RPMLimit,
	}
	for name, value := range map[string]string{
		"key_alias": request.KeyAlias, "user_id": request.UserID,
		"team_id": request.TeamID, "budget_duration": request.BudgetDuration,
	} {
		body[name] = nil
		if value != "" {
			body[name] = value
		}
	}
	if request.ObjectPermission != nil {
		permission := *request.ObjectPermission
		if permission.MCPToolsets == nil {
			permission.MCPToolsets = []string{}
		}
		body["object_permission"] = permission
	}
	if updateDuration {
		body["duration"] = nil
		if request.Duration != "" {
			body["duration"] = request.Duration
		}
	}
	return c.do(ctx, http.MethodPost, "/key/update", body, nil)
}

// DeleteVirtualKey deletes a virtual key (POST /key/delete).
func (c *Client) DeleteVirtualKey(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodPost, "/key/delete", map[string][]string{"keys": {key}}, nil)
}

// CreateTeam creates a LiteLLM team (POST /team/new).
func (c *Client) CreateTeam(ctx context.Context, team TeamRequest) (Team, error) {
	if team.ObjectPermission != nil && team.ObjectPermission.MCPServers == nil {
		team.ObjectPermission = &TeamObjectPermission{MCPServers: []string{}}
	}
	var out Team
	err := c.do(ctx, http.MethodPost, "/team/new", team, &out)
	return out, err
}

// UpdateTeam updates a LiteLLM team (POST /team/update).
func (c *Client) UpdateTeam(ctx context.Context, team TeamRequest) error {
	if team.ObjectPermission != nil && team.ObjectPermission.MCPServers == nil {
		team.ObjectPermission = &TeamObjectPermission{MCPServers: []string{}}
	}
	return c.do(ctx, http.MethodPost, "/team/update", team, nil)
}

// DeleteTeam deletes a LiteLLM team and its associated keys (POST /team/delete).
func (c *Client) DeleteTeam(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/team/delete", map[string][]string{"team_ids": {id}}, nil)
}

// GetTeam returns a LiteLLM team (GET /team/info).
func (c *Client) GetTeam(ctx context.Context, id string) (Team, error) {
	var out struct {
		TeamInfo Team `json:"team_info"`
	}
	err := c.do(ctx, http.MethodGet, "/team/info?team_id="+url.QueryEscape(id), nil, &out)
	return out.TeamInfo, err
}

// AddTeamMember adds a member to a LiteLLM team (POST /team/member_add).
func (c *Client) AddTeamMember(ctx context.Context, id string, m TeamMember) error {
	return c.do(ctx, http.MethodPost, "/team/member_add", map[string]any{teamIDJSONKey: id, "member": m}, nil)
}

// DeleteTeamMember removes a member from a LiteLLM team (POST /team/member_delete).
func (c *Client) DeleteTeamMember(ctx context.Context, id string, m TeamMember) error {
	return c.do(ctx, http.MethodPost, "/team/member_delete", map[string]string{teamIDJSONKey: id, "user_id": m.UserID, "user_email": m.UserEmail}, nil)
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

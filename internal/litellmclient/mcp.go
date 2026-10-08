package litellmclient

import (
	"context"
	"net/http"
	"net/url"
)

// ListMCPServers returns the registered MCP servers, with credentials redacted by LiteLLM.
func (c *Client) ListMCPServers(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	err := c.do(ctx, http.MethodGet, "/v1/mcp/server", nil, &out)
	return out, err
}

// CreateMCPServer registers a database-backed MCP server.
func (c *Client) CreateMCPServer(ctx context.Context, server map[string]any) error {
	return c.do(ctx, http.MethodPost, "/v1/mcp/server", server, nil, mcpSecrets(server)...)
}

// UpdateMCPServer updates an MCP server and its credentials in place.
func (c *Client) UpdateMCPServer(ctx context.Context, server map[string]any) error {
	return c.do(ctx, http.MethodPut, "/v1/mcp/server", server, nil, mcpSecrets(server)...)
}

// DeleteMCPServer deletes a database-backed MCP server.
func (c *Client) DeleteMCPServer(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/mcp/server/"+url.PathEscape(id), nil, nil)
}

func mcpSecrets(server map[string]any) []string {
	var secrets []string
	for _, field := range []string{"credentials", "env", "static_headers"} {
		values, _ := server[field].(map[string]any)
		for _, value := range values {
			if s, ok := value.(string); ok {
				secrets = append(secrets, s)
			}
		}
	}
	return secrets
}

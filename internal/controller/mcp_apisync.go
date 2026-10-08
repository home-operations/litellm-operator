package controller

import (
	"context"
	"encoding/json"
	"fmt"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/litellmclient"
)

const mcpProxyOwnerKey = "operator_proxy"

func (r *LiteLLMProxyReconciler) syncMCPServersViaAPI(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy, servers []litellmv1alpha1.LiteLLMMCPServer) error {
	if len(servers) == 0 && proxy.Status.MCPConfigHash == "" {
		return nil
	}
	if proxy.Spec.APIAccess == nil {
		return fmt.Errorf("MCP API sync requires spec.apiAccess")
	}
	owner := fmt.Sprintf("%s/%s/%s", proxy.Namespace, proxy.Name, proxy.UID)
	desired := make(map[string]map[string]any, len(servers))
	aliases := make(map[string]struct{}, len(servers))
	for i := range servers {
		server := &servers[i]
		alias := server.ServerAlias()
		if _, duplicate := aliases[alias]; duplicate {
			return fmt.Errorf("mcp server %q reuses alias %q", server.Name, alias)
		}
		aliases[alias] = struct{}{}
		entry, err := r.mcpAPIEntry(ctx, server)
		if err != nil {
			return fmt.Errorf("mcp server %q: %w", server.Name, err)
		}
		info, _ := entry["mcp_info"].(map[string]any)
		if info == nil {
			info = map[string]any{}
		}
		info[managedByKey], info[mcpProxyOwnerKey] = managedByValue, owner
		entry["mcp_info"] = info
		id := "operator-" + hashString(fmt.Sprintf("%s/%s/%s/%s", owner, server.Namespace, server.Name, server.UID))
		entry["server_id"] = id
		desired[id] = entry
	}
	encoded, err := json.Marshal(desired)
	if err != nil {
		return fmt.Errorf("encode mcp configuration: %w", err)
	}
	hash := hashString(string(encoded))
	key, err := readSecretKey(ctx, r.Client, proxy.Namespace, proxy.Spec.APIAccess.MasterKeyRef)
	if err != nil {
		return err
	}
	endpoint := proxy.Spec.APIAccess.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("http://%s.%s.svc:%d", proxy.Name, proxy.Namespace, servicePort(proxy))
	}
	c := litellmclient.New(endpoint, key, nil)
	existing, err := c.ListMCPServers(ctx)
	if err != nil {
		return err
	}
	managed := map[string]map[string]any{}
	for _, entry := range existing {
		info, _ := entry["mcp_info"].(map[string]any)
		if info[managedByKey] == managedByValue && info[mcpProxyOwnerKey] == owner {
			id, _ := entry["server_id"].(string)
			managed[id] = entry
		}
	}
	for id := range managed {
		if _, keep := desired[id]; !keep {
			if err := c.DeleteMCPServer(ctx, id); err != nil {
				return err
			}
		}
	}
	for id, entry := range desired {
		if current, ok := managed[id]; ok {
			if proxy.Status.MCPConfigHash == hash && mcpConfigEqual(entry, current) {
				continue
			}
			if err := c.UpdateMCPServer(ctx, entry); err != nil {
				return err
			}
		} else if err := c.CreateMCPServer(ctx, entry); err != nil {
			return err
		}
	}
	proxy.Status.MCPConfigHash = hash
	return nil
}

func (r *LiteLLMProxyReconciler) mcpAPIEntry(ctx context.Context, server *litellmv1alpha1.LiteLLMMCPServer) (map[string]any, error) {
	entry, err := decodeRaw(server.Spec.Params)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		entry = map[string]any{}
	}
	entry["server_name"], entry["alias"] = server.ServerAlias(), server.ServerAlias()
	entry["url"] = server.ResolvedServerURL()
	transport := server.Spec.Transport
	if transport == "" {
		transport, _ = entry["transport"].(string)
	}
	if transport == "" {
		transport = "sse"
		if server.Spec.Workload != nil {
			transport = "http"
		}
	}
	entry["transport"] = transport
	if server.Spec.AuthType != "" {
		entry["auth_type"] = server.Spec.AuthType
	} else if _, ok := entry["auth_type"]; !ok {
		entry["auth_type"] = nil
	}
	credentials, _ := entry["credentials"].(map[string]any)
	if credentials == nil {
		credentials = map[string]any{}
	}
	for _, field := range []string{"auth_value", "client_id", "client_secret", "scopes", "aws_access_key_id", "aws_secret_access_key", "aws_session_token", "aws_region_name", "aws_service_name", "aws_role_name", "aws_session_name"} {
		if value, ok := entry[field]; ok {
			credentials[field] = value
			delete(entry, field)
		}
	}
	if value, ok := entry["authentication_token"]; ok {
		credentials["auth_value"] = value
		delete(entry, "authentication_token")
	}
	if server.Spec.AuthTokenRef != nil {
		value, err := readSecretKey(ctx, r.Client, server.Namespace, *server.Spec.AuthTokenRef)
		if err != nil {
			return nil, err
		}
		credentials["auth_value"] = value
	} else if _, ok := credentials["auth_value"]; !ok {
		credentials["auth_value"] = nil
	}
	entry["credentials"] = credentials
	return entry, nil
}

func mcpConfigEqual(desired, existing map[string]any) bool {
	for key, value := range desired {
		// LiteLLM redacts credentials on read; status tracks successful writes.
		if key == "credentials" {
			continue
		}
		if key == "mcp_info" {
			info, _ := value.(map[string]any)
			current, _ := existing[key].(map[string]any)
			if !subsetEqual(info, current) {
				return false
			}
		} else if !jsonEqual(value, existing[key]) {
			return false
		}
	}
	return true
}

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const mcpTokenEnvPrefix = "LITELLM_MCPTOKEN_"

// LiteLLMMCPServerSpec defines a single config.yaml mcp_servers entry.
type LiteLLMMCPServerSpec struct {
	// Alias is the key under mcp_servers this server is registered as. Defaults
	// to the resource name when empty.
	// +optional
	Alias string `json:"alias,omitempty"`

	// ProxyRef explicitly binds this server to a LiteLLMProxy by name in the same
	// namespace. When set it takes precedence over a proxy's modelSelector.
	// +optional
	ProxyRef string `json:"proxyRef,omitempty"`

	// URL of the MCP server.
	// +optional
	URL string `json:"url,omitempty"`

	// Transport the server speaks.
	// +kubebuilder:validation:Enum=sse;http;stdio
	// +optional
	Transport string `json:"transport,omitempty"`

	// AuthType configures how the gateway authenticates to the server.
	// +optional
	AuthType string `json:"authType,omitempty"`

	// AuthTokenRef sources the server's authentication_token from a Secret.
	// +optional
	AuthTokenRef *SecretKeyRef `json:"authTokenRef,omitempty"`

	// Params holds any further mcp_servers fields (extra_headers, allowed_tools,
	// oauth/aws/stdio settings, ...), merged under the typed fields.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Params *runtime.RawExtension `json:"params,omitempty"`
}

// LiteLLMMCPServerStatus reports the observed state of the MCP server.
type LiteLLMMCPServerStatus struct {
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=llmcp
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.url`
// +kubebuilder:printcolumn:name="Transport",type=string,JSONPath=`.spec.transport`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LiteLLMMCPServer is a single MCP server exposed by one or more LiteLLMProxy instances.
type LiteLLMMCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMMCPServerSpec   `json:"spec,omitempty"`
	Status LiteLLMMCPServerStatus `json:"status,omitempty"`
}

// ServerAlias is the mcp_servers map key for this server.
func (s *LiteLLMMCPServer) ServerAlias() string {
	if s.Spec.Alias != "" {
		return s.Spec.Alias
	}
	return s.Name
}

// AuthTokenEnvVarName is the env var carrying this server's secret-backed auth token.
func (s *LiteLLMMCPServer) AuthTokenEnvVarName() string {
	return mcpTokenEnvPrefix + sanitizeEnvVar(s.Name)
}

// +kubebuilder:object:root=true

// LiteLLMMCPServerList contains a list of LiteLLMMCPServer.
type LiteLLMMCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMMCPServer `json:"items"`
}

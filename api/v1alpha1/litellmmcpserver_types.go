package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	mcpTokenEnvPrefix = "LITELLM_MCPTOKEN_"

	defaultMCPWorkloadPort int32 = 8080
	defaultMCPWorkloadPath       = "/mcp"
)

// MCPWorkloadSpec makes the operator run the MCP server as a Deployment +
// Service and derive its url from that Service.
type MCPWorkloadSpec struct {
	// Image is the container image.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Replicas of the Deployment. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ServiceAccountName is the pod's ServiceAccount. Set this to grant the
	// workload the RBAC bound to that ServiceAccount. Defaults to the namespace
	// default ServiceAccount when empty.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// AutomountServiceAccountToken controls whether the ServiceAccount token is
	// mounted into the pod. Defaults to true (Kubernetes default).
	// +optional
	AutomountServiceAccountToken *bool `json:"automountServiceAccountToken,omitempty"`

	// PodAnnotations are added to the pod template metadata.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// PodLabels are merged into the pod template labels, on top of the labels the
	// operator manages. They cannot override the selector labels.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// Command overrides the image entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args passed to the container.
	// +optional
	Args []string `json:"args,omitempty"`

	// Port the server listens on; also the Service port and the derived-url
	// port. Defaults to 8080.
	// +optional
	Port int32 `json:"port,omitempty"`

	// Path appended to the derived url. Defaults to "/mcp".
	// +optional
	Path string `json:"path,omitempty"`

	// Env for the container; supports valueFrom for secret-backed values.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom sources for the container.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Resources for the container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// SecurityContext is the container's security context (runAsNonRoot,
	// capabilities, readOnlyRootFilesystem, ...). Required under a Pod Security
	// "restricted" namespace.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// PodSecurityContext is the pod-level security context (runAsUser, fsGroup,
	// seccompProfile, ...).
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// NodeSelector constrains the pod to nodes with matching labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations allow the pod to schedule onto tainted nodes.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity constrains pod scheduling (node/pod affinity and anti-affinity).
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// VolumeMounts for the container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Volumes for the pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`
}

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

	// URL of an external MCP server. Mutually exclusive with workload.
	// +optional
	URL string `json:"url,omitempty"`

	// Transport the server speaks. Defaults to http when a workload is set.
	// +kubebuilder:validation:Enum=sse;http;stdio
	// +optional
	Transport string `json:"transport,omitempty"`

	// AuthType configures how the gateway authenticates to the server.
	// +optional
	AuthType string `json:"authType,omitempty"`

	// AuthTokenRef sources the server's authentication_token from a Secret.
	// +optional
	AuthTokenRef *SecretKeyRef `json:"authTokenRef,omitempty"`

	// Workload, when set, makes the operator run this MCP server as a Deployment
	// + Service and derive the url from it. Mutually exclusive with url.
	// +optional
	Workload *MCPWorkloadSpec `json:"workload,omitempty"`

	// Params holds any further mcp_servers fields (extra_headers, allowed_tools,
	// oauth/aws/stdio settings, ...), merged under the typed fields.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Params *runtime.RawExtension `json:"params,omitempty"`
}

// LiteLLMMCPServerStatus reports the observed state of the MCP server.
type LiteLLMMCPServerStatus struct {
	// ResolvedURL is the url the gateway uses to reach this server: the derived
	// workload url, or spec.url.
	// +optional
	ResolvedURL string `json:"resolvedURL,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=llmcp
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.resolvedURL`
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

// WorkloadPort returns the configured workload port or the default.
func (s *LiteLLMMCPServer) WorkloadPort() int32 {
	if s.Spec.Workload != nil && s.Spec.Workload.Port != 0 {
		return s.Spec.Workload.Port
	}
	return defaultMCPWorkloadPort
}

// WorkloadPath returns the configured derived-url path or the default.
func (s *LiteLLMMCPServer) WorkloadPath() string {
	if s.Spec.Workload != nil && s.Spec.Workload.Path != "" {
		return s.Spec.Workload.Path
	}
	return defaultMCPWorkloadPath
}

// WorkloadURL is the in-cluster url of the Service the operator runs for this
// server's workload.
func (s *LiteLLMMCPServer) WorkloadURL() string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s", s.Name, s.Namespace, s.WorkloadPort(), s.WorkloadPath())
}

// ResolvedServerURL is the url the gateway uses to reach this server.
func (s *LiteLLMMCPServer) ResolvedServerURL() string {
	if s.Spec.Workload != nil {
		return s.WorkloadURL()
	}
	return s.Spec.URL
}

// +kubebuilder:object:root=true

// LiteLLMMCPServerList contains a list of LiteLLMMCPServer.
type LiteLLMMCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMMCPServer `json:"items"`
}

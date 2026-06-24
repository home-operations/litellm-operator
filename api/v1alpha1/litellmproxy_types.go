package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RouteParentRef identifies the Gateway (or other parent) the HTTPRoute attaches to.
type RouteParentRef struct {
	// Name of the parent Gateway.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the parent Gateway. Defaults to the proxy's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// SectionName pins the route to a specific Gateway listener.
	// +optional
	SectionName string `json:"sectionName,omitempty"`
}

// ProxyRoute describes the HTTPRoute the operator creates for the proxy.
type ProxyRoute struct {
	// Hostnames the route matches.
	// +kubebuilder:validation:MinItems=1
	Hostnames []string `json:"hostnames"`

	// ParentRefs are the Gateways the route attaches to.
	// +kubebuilder:validation:MinItems=1
	ParentRefs []RouteParentRef `json:"parentRefs"`
}

// CallbackSpec configures litellm callbacks. Success/Failure/Callbacks set the
// matching litellm_settings lists; Settings maps to the top-level callback_settings.
type CallbackSpec struct {
	// Success callbacks (litellm_settings.success_callback), e.g. ["prometheus","langfuse"].
	// +optional
	// +listType=atomic
	Success []string `json:"success,omitempty"`

	// Failure callbacks (litellm_settings.failure_callback).
	// +optional
	// +listType=atomic
	Failure []string `json:"failure,omitempty"`

	// Callbacks run on both success and failure (litellm_settings.callbacks).
	// +optional
	// +listType=atomic
	Callbacks []string `json:"callbacks,omitempty"`

	// Settings maps to the top-level callback_settings block (per-callback config).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Settings *runtime.RawExtension `json:"settings,omitempty"`
}

// APIAccessSpec configures the admin API connection used when applyMode is "api".
type APIAccessSpec struct {
	// Endpoint of the proxy admin API. Defaults to the operator-managed Service,
	// i.e. http://<proxy>.<namespace>.svc:<service port>.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// MasterKeyRef sources the proxy master key used to authenticate admin calls.
	// +kubebuilder:validation:Required
	MasterKeyRef SecretKeyRef `json:"masterKeyRef"`
}

// ProxyServiceSpec configures the Service fronting the proxy.
type ProxyServiceSpec struct {
	// Type of Service to create.
	// +kubebuilder:default=ClusterIP
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Port the Service exposes; the proxy container always listens on 4000.
	// +kubebuilder:default=4000
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`
}

// LiteLLMProxySpec defines a managed LiteLLM proxy and the models it serves.
type LiteLLMProxySpec struct {
	// Image is the proxy container image.
	// +kubebuilder:default="ghcr.io/berriai/litellm:main-stable"
	// +optional
	Image string `json:"image,omitempty"`

	// Replicas of the proxy Deployment.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ApplyMode selects how models, guardrails and MCP servers reach the proxy:
	// "file" renders them into config.yaml and rolls the Deployment on change;
	// "api" pushes them to the proxy's DB-backed admin API live, with no restart
	// (requires the proxy to run in DB mode, i.e. Postgres, and apiAccess set).
	// +kubebuilder:validation:Enum=file;api
	// +kubebuilder:default=file
	// +optional
	ApplyMode string `json:"applyMode,omitempty"`

	// APIAccess configures the admin API connection used when applyMode is "api".
	// +optional
	APIAccess *APIAccessSpec `json:"apiAccess,omitempty"`

	// ModelSelector selects which LiteLLMModel resources (in this namespace) this
	// proxy serves. When omitted, the proxy adopts every LiteLLMModel in its
	// namespace that does not pin a different proxy via spec.proxyRef.
	// +optional
	ModelSelector *metav1.LabelSelector `json:"modelSelector,omitempty"`

	// Route, when set, makes the operator create and own a Gateway API HTTPRoute
	// that fronts the proxy Service.
	// +optional
	Route *ProxyRoute `json:"route,omitempty"`

	// GeneralSettings maps verbatim to the config.yaml general_settings block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	GeneralSettings *runtime.RawExtension `json:"generalSettings,omitempty"`

	// RouterSettings maps verbatim to the config.yaml router_settings block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	RouterSettings *runtime.RawExtension `json:"routerSettings,omitempty"`

	// LitellmSettings maps verbatim to the config.yaml litellm_settings block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	LitellmSettings *runtime.RawExtension `json:"litellmSettings,omitempty"`

	// Callbacks configures litellm logging/observability callbacks.
	// +optional
	Callbacks *CallbackSpec `json:"callbacks,omitempty"`

	// EnvironmentVariables maps to the top-level environment_variables block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	EnvironmentVariables *runtime.RawExtension `json:"environmentVariables,omitempty"`

	// CredentialList maps to the top-level credential_list block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	CredentialList *runtime.RawExtension `json:"credentialList,omitempty"`

	// DefaultVertexConfig maps to the top-level default_vertex_config block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	DefaultVertexConfig *runtime.RawExtension `json:"defaultVertexConfig,omitempty"`

	// FilesSettings maps to the top-level files_settings block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	FilesSettings *runtime.RawExtension `json:"filesSettings,omitempty"`

	// AssistantSettings maps to the top-level assistant_settings block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	AssistantSettings *runtime.RawExtension `json:"assistantSettings,omitempty"`

	// FinetuneSettings maps to the top-level finetune_settings block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	FinetuneSettings *runtime.RawExtension `json:"finetuneSettings,omitempty"`

	// Prompts maps to the top-level prompts block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Prompts *runtime.RawExtension `json:"prompts,omitempty"`

	// VectorStoreRegistry maps to the top-level vector_store_registry block.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	VectorStoreRegistry *runtime.RawExtension `json:"vectorStoreRegistry,omitempty"`

	// ExtraConfig is merged into the top level of the rendered config.yaml as a
	// final catch-all for any key without a dedicated field. The generated
	// model_list, guardrails, mcp_servers and the typed blocks take precedence.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	ExtraConfig *runtime.RawExtension `json:"extraConfig,omitempty"`

	// Env is injected into the proxy container (e.g. REDIS_HOST, master key).
	// +optional
	// +listType=atomic
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom sources whole Secrets/ConfigMaps as env into the proxy container.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Service configures the Service fronting the proxy.
	// +optional
	Service ProxyServiceSpec `json:"service,omitempty"`

	// Resources sets the proxy container resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// LivenessProbe overrides the proxy liveness probe. Defaults to an HTTP GET
	// of /health/liveliness on the proxy port.
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// ReadinessProbe overrides the proxy readiness probe. Defaults to an HTTP GET
	// of /health/readiness on the proxy port.
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
}

// LiteLLMProxyStatus reports the observed state of the proxy.
type LiteLLMProxyStatus struct {
	// Conditions represent the latest observations of the proxy's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ConfigHash is the sha256 of the rendered config.yaml currently applied.
	// +optional
	ConfigHash string `json:"configHash,omitempty"`

	// ObservedModels is the number of LiteLLMModel resources folded into the config.
	// +optional
	ObservedModels int32 `json:"observedModels,omitempty"`

	// ReadyReplicas mirrors the managed Deployment's ready replica count.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=llproxy
// +kubebuilder:printcolumn:name="Models",type=integer,JSONPath=`.status.observedModels`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LiteLLMProxy owns a Deployment, Service, and ConfigMap rendered from the
// LiteLLMModel resources its ModelSelector matches.
type LiteLLMProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMProxySpec   `json:"spec,omitempty"`
	Status LiteLLMProxyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiteLLMProxyList contains a list of LiteLLMProxy.
type LiteLLMProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMProxy `json:"items"`
}

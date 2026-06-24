package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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

	// ModelSelector selects which LiteLLMModel resources (in this namespace) this
	// proxy serves. An empty selector matches nothing; set matchLabels to opt models in.
	// +kubebuilder:validation:Required
	ModelSelector metav1.LabelSelector `json:"modelSelector"`

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

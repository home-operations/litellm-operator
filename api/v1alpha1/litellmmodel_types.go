package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	apiKeyEnvPrefix  = "LITELLM_MODELKEY_"
	apiBaseEnvPrefix = "LITELLM_MODELBASE_"
)

// SecretKeyRef points at a single key within a Secret in the same namespace as
// the LiteLLMModel. The operator wires it into the proxy Deployment as an
// environment variable and references it from config.yaml via os.environ, so
// the secret value never lands in the rendered ConfigMap.
type SecretKeyRef struct {
	// Name of the Secret.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key within the Secret holding the value.
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

// LiteLLMParams maps to a single model_list[].litellm_params entry. The common
// fields are typed; everything else (provider-specific knobs) goes in Additional.
type LiteLLMParams struct {
	// Model is the underlying provider model identifier, e.g. "openai/glm-5.2".
	// +kubebuilder:validation:Required
	Model string `json:"model"`

	// APIBase is the provider base URL. Mutually exclusive with APIBaseRef.
	// +optional
	APIBase string `json:"apiBase,omitempty"`

	// APIBaseRef sources the provider base URL from a Secret. Mutually exclusive with APIBase.
	// +optional
	APIBaseRef *SecretKeyRef `json:"apiBaseRef,omitempty"`

	// APIKeyRef sources the provider API key from a Secret. Mutually exclusive with APIKey.
	// +optional
	APIKeyRef *SecretKeyRef `json:"apiKeyRef,omitempty"`

	// APIKey is a literal or os.environ/VAR API key. Prefer APIKeyRef for real secrets.
	// +optional
	APIKey string `json:"apiKey,omitempty"`

	// APIVersion for providers that require it (e.g. Azure).
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// DropParams drops request params the provider does not support instead of erroring.
	// +optional
	DropParams *bool `json:"dropParams,omitempty"`

	// RPM caps requests-per-minute for this deployment.
	// +optional
	RPM *int64 `json:"rpm,omitempty"`

	// TPM caps tokens-per-minute for this deployment.
	// +optional
	TPM *int64 `json:"tpm,omitempty"`

	// Additional holds any further litellm_params, merged verbatim under the typed
	// fields (which win on conflict).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Additional *runtime.RawExtension `json:"additional,omitempty"`
}

// ModelInfo maps to model_list[].model_info. The common fields are typed and
// validated; anything else goes in Extra.
type ModelInfo struct {
	// MaxTokens is the model's total token budget (max_tokens).
	// +optional
	MaxTokens *int64 `json:"maxTokens,omitempty"`

	// MaxInputTokens is the context window size (max_input_tokens).
	// +optional
	MaxInputTokens *int64 `json:"maxInputTokens,omitempty"`

	// MaxOutputTokens is the maximum completion length (max_output_tokens).
	// +optional
	MaxOutputTokens *int64 `json:"maxOutputTokens,omitempty"`

	// Mode is the endpoint mode, e.g. "chat", "messages", "embedding".
	// +optional
	Mode string `json:"mode,omitempty"`

	// SupportsFunctionCalling advertises tool/function-calling support.
	// +optional
	SupportsFunctionCalling *bool `json:"supportsFunctionCalling,omitempty"`

	// SupportsPromptCaching advertises prompt-caching support.
	// +optional
	SupportsPromptCaching *bool `json:"supportsPromptCaching,omitempty"`

	// SupportsVision advertises image-input support.
	// +optional
	SupportsVision *bool `json:"supportsVision,omitempty"`

	// Extra holds any further model_info keys, merged verbatim under the typed
	// fields (which win on conflict).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Extra *runtime.RawExtension `json:"extra,omitempty"`
}

// LiteLLMModelSpec defines a single proxy model_list entry.
type LiteLLMModelSpec struct {
	// ModelName is the public name clients call (model_list[].model_name).
	// +kubebuilder:validation:Required
	ModelName string `json:"modelName"`

	// ProxyRef explicitly binds this model to a LiteLLMProxy by name in the same
	// namespace. When set it takes precedence over any proxy's modelSelector.
	// +optional
	ProxyRef string `json:"proxyRef,omitempty"`

	// Params is the litellm_params block for this model.
	// +kubebuilder:validation:Required
	Params LiteLLMParams `json:"params"`

	// Info is the typed model_info block for this model.
	// +optional
	Info *ModelInfo `json:"info,omitempty"`
}

// LiteLLMModelStatus reports which proxies currently include this model.
type LiteLLMModelStatus struct {
	// Conditions represent the latest observations of the model's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=llmodel
// +kubebuilder:printcolumn:name="Model Name",type=string,JSONPath=`.spec.modelName`
// +kubebuilder:printcolumn:name="Provider Model",type=string,JSONPath=`.spec.params.model`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LiteLLMModel is a single model served by one or more LiteLLMProxy instances.
type LiteLLMModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMModelSpec   `json:"spec,omitempty"`
	Status LiteLLMModelStatus `json:"status,omitempty"`
}

// APIKeyEnvVarName is the deterministic env var name the operator injects into
// the proxy Deployment to carry this model's secret-backed API key, referenced
// from config.yaml as os.environ/<name>. Derived from the resource name (a
// DNS-1123 subdomain), e.g. "minimax-m3" -> "LITELLM_MODELKEY_MINIMAX_M3".
func (m *LiteLLMModel) APIKeyEnvVarName() string {
	return apiKeyEnvPrefix + sanitizeEnvVar(m.Name)
}

// APIBaseEnvVarName is the deterministic env var name for this model's
// secret-backed API base, e.g. "qwen" -> "LITELLM_MODELBASE_QWEN".
func (m *LiteLLMModel) APIBaseEnvVarName() string {
	return apiBaseEnvPrefix + sanitizeEnvVar(m.Name)
}

func sanitizeEnvVar(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, name)
}

// +kubebuilder:object:root=true

// LiteLLMModelList contains a list of LiteLLMModel.
type LiteLLMModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMModel `json:"items"`
}

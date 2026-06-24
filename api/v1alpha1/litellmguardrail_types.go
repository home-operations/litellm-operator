package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	guardrailKeyEnvPrefix  = "LITELLM_GUARDRAILKEY_"
	guardrailBaseEnvPrefix = "LITELLM_GUARDRAILBASE_"
)

// LiteLLMGuardrailSpec defines a single config.yaml guardrails[] entry.
type LiteLLMGuardrailSpec struct {
	// GuardrailName is the name clients reference (guardrails[].guardrail_name).
	// +kubebuilder:validation:Required
	GuardrailName string `json:"guardrailName"`

	// ProxyRef explicitly binds this guardrail to a LiteLLMProxy by name in the
	// same namespace. When set it takes precedence over a proxy's modelSelector.
	// +optional
	ProxyRef string `json:"proxyRef,omitempty"`

	// Guardrail is the provider integration, e.g. "aporia", "bedrock", "presidio".
	// +kubebuilder:validation:Required
	Guardrail string `json:"guardrail"`

	// Mode is when the guardrail runs.
	// +kubebuilder:validation:Enum=pre_call;post_call;during_call;logging_only
	// +optional
	Mode string `json:"mode,omitempty"`

	// DefaultOn applies the guardrail to every request unless overridden.
	// +optional
	DefaultOn *bool `json:"defaultOn,omitempty"`

	// APIBase is the guardrail provider base URL. Mutually exclusive with APIBaseRef.
	// +optional
	APIBase string `json:"apiBase,omitempty"`

	// APIBaseRef sources the base URL from a Secret. Mutually exclusive with APIBase.
	// +optional
	APIBaseRef *SecretKeyRef `json:"apiBaseRef,omitempty"`

	// APIKey is a literal or os.environ/VAR key. Mutually exclusive with APIKeyRef.
	// +optional
	APIKey string `json:"apiKey,omitempty"`

	// APIKeyRef sources the provider API key from a Secret. Mutually exclusive with APIKey.
	// +optional
	APIKeyRef *SecretKeyRef `json:"apiKeyRef,omitempty"`

	// Params holds any further guardrail litellm_params (provider-specific knobs),
	// merged under the typed fields (which win on conflict).
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Params *runtime.RawExtension `json:"params,omitempty"`

	// Info maps verbatim to guardrails[].guardrail_info.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	// +optional
	Info *runtime.RawExtension `json:"info,omitempty"`
}

// LiteLLMGuardrailStatus reports the observed state of the guardrail.
type LiteLLMGuardrailStatus struct {
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=llguardrail
// +kubebuilder:printcolumn:name="Guardrail",type=string,JSONPath=`.spec.guardrail`
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LiteLLMGuardrail is a single guardrail served by one or more LiteLLMProxy instances.
type LiteLLMGuardrail struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMGuardrailSpec   `json:"spec,omitempty"`
	Status LiteLLMGuardrailStatus `json:"status,omitempty"`
}

// APIKeyEnvVarName is the env var carrying this guardrail's secret-backed API key.
func (g *LiteLLMGuardrail) APIKeyEnvVarName() string {
	return guardrailKeyEnvPrefix + sanitizeEnvVar(g.Name)
}

// APIBaseEnvVarName is the env var carrying this guardrail's secret-backed base URL.
func (g *LiteLLMGuardrail) APIBaseEnvVarName() string {
	return guardrailBaseEnvPrefix + sanitizeEnvVar(g.Name)
}

// +kubebuilder:object:root=true

// LiteLLMGuardrailList contains a list of LiteLLMGuardrail.
type LiteLLMGuardrailList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMGuardrail `json:"items"`
}

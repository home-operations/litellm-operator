package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LiteLLMVirtualKeySpec defines a virtual key managed through a LiteLLMProxy.
type LiteLLMVirtualKeySpec struct {
	// ProxyRef is the LiteLLMProxy whose admin API creates and deletes this key.
	// +kubebuilder:validation:Required
	ProxyRef string `json:"proxyRef"`

	// ProxyNamespace is the proxy's namespace. Defaults to this resource's namespace.
	// +optional
	ProxyNamespace string `json:"proxyNamespace,omitempty"`

	// SecretName is the name of the Secret managed by the operator containing the generated key.
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// SecretKey is the key in SecretName containing the generated virtual key.
	// +kubebuilder:default=key
	// +optional
	SecretKey string `json:"secretKey,omitempty"`

	// SecretAnnotations are stamped on the generated Secret, so it can carry
	// annotations other controllers act on, such as kubernetes-reflector's
	// reflection-allowed on the source Secret. The operator manages only the keys
	// named here: annotations written by anything else are left alone, and a key
	// dropped from this map is removed from the Secret on the next reconcile.
	// The litellm.home-operations.com/managed- prefix is reserved for the
	// annotations recording which keys the operator manages.
	// +kubebuilder:validation:XValidation:rule="self.all(k, !k.startsWith('litellm.home-operations.com/managed-'))",message="secretAnnotations must not use the reserved litellm.home-operations.com/managed- key prefix"
	// +optional
	SecretAnnotations map[string]string `json:"secretAnnotations,omitempty"`

	// SecretLabels are stamped on the generated Secret, under the same
	// operator-manages-only-these-keys rule as SecretAnnotations.
	// +kubebuilder:validation:XValidation:rule="self.all(k, !k.startsWith('litellm.home-operations.com/managed-'))",message="secretLabels must not use the reserved litellm.home-operations.com/managed- key prefix"
	// +optional
	SecretLabels map[string]string `json:"secretLabels,omitempty"`

	// KeyAlias is a human-readable LiteLLM alias for this key.
	// +optional
	KeyAlias string `json:"keyAlias,omitempty"`

	// Models lists the models this key may call. An empty list permits all models.
	// +listType=atomic
	// +optional
	Models []string `json:"models,omitempty"`

	// MCPToolsets lists the LiteLLM MCP toolset IDs assigned to this key.
	// An empty list clears key-level toolset assignments; other MCP permissions still apply.
	// +listType=atomic
	// +optional
	MCPToolsets []string `json:"mcpToolsets,omitempty"`

	// Aliases maps model aliases to their target model names.
	// +optional
	Aliases map[string]string `json:"aliases,omitempty"`

	// UserID associates this key with a LiteLLM user.
	// +optional
	UserID string `json:"userID,omitempty"`

	// TeamID associates this key with a LiteLLM team.
	// +optional
	TeamID string `json:"teamID,omitempty"`

	// Duration is how long the key remains valid, such as "30d".
	// +optional
	Duration string `json:"duration,omitempty"`

	// MaxBudget is the maximum spend allowed for this key as a decimal string.
	// +optional
	MaxBudget string `json:"maxBudget,omitempty"`

	// BudgetDuration resets the budget after this duration, such as "30d".
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`

	// MaxParallelRequests caps concurrent requests made with this key.
	// +optional
	MaxParallelRequests *int64 `json:"maxParallelRequests,omitempty"`

	// TPMLimit caps tokens per minute for this key.
	// +optional
	TPMLimit *int64 `json:"tpmLimit,omitempty"`

	// RPMLimit caps requests per minute for this key.
	// +optional
	RPMLimit *int64 `json:"rpmLimit,omitempty"`

	// Metadata stores additional string metadata on the LiteLLM key.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// LiteLLMVirtualKeyStatus reports the observed state of the virtual key.
type LiteLLMVirtualKeyStatus struct {
	// Conditions represent the latest observations of the virtual key's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=llkey
// +kubebuilder:printcolumn:name="Proxy",type=string,JSONPath=`.spec.proxyRef`
// +kubebuilder:printcolumn:name="Secret",type=string,JSONPath=`.spec.secretName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// LiteLLMVirtualKey is a virtual key generated and deleted through LiteLLM's admin API.
type LiteLLMVirtualKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiteLLMVirtualKeySpec   `json:"spec,omitempty"`
	Status LiteLLMVirtualKeyStatus `json:"status,omitempty"`
}

// SecretDataKey returns the configured Secret key, defaulting to "key".
func (k *LiteLLMVirtualKey) SecretDataKey() string {
	if k.Spec.SecretKey == "" {
		return "key"
	}
	return k.Spec.SecretKey
}

// +kubebuilder:object:root=true

// LiteLLMVirtualKeyList contains a list of LiteLLMVirtualKey.
type LiteLLMVirtualKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMVirtualKey `json:"items"`
}

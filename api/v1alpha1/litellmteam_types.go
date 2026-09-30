package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type LiteLLMTeamMember struct {
	// UserID identifies an existing LiteLLM user. Either UserID or UserEmail is required.
	// +optional
	UserID string `json:"userID,omitempty"`
	// UserEmail identifies or creates a LiteLLM user. Either UserID or UserEmail is required.
	// +optional
	UserEmail string `json:"userEmail,omitempty"`
	// Role is the member's role within the team.
	// +kubebuilder:validation:Enum=admin;user
	Role string `json:"role"`
}

type LiteLLMTeamSpec struct {
	// ProxyRef is the LiteLLMProxy whose admin API manages this team.
	ProxyRef string `json:"proxyRef"`
	// TeamID is the LiteLLM team ID. It defaults to the Kubernetes resource name.
	// +optional
	TeamID string `json:"teamID,omitempty"`
	// Alias is a human-readable LiteLLM alias.
	// +optional
	Alias string `json:"alias,omitempty"`
	// Members is the complete set of team members and their roles.
	// +listType=atomic
	// +optional
	Members []LiteLLMTeamMember `json:"members,omitempty"`
	// Models limits models available to keys in this team. An empty list permits all models.
	// +listType=atomic
	// +optional
	Models []string `json:"models,omitempty"`
	// MCPServers lists the LiteLLM MCP server IDs or aliases assigned to this team.
	// An empty list clears team-level server assignments; other MCP permissions still apply.
	// +listType=atomic
	// +optional
	MCPServers []string `json:"mcpServers,omitempty"`
	// MaxBudget is the maximum spend allowed, expressed as a decimal string.
	// +optional
	MaxBudget string `json:"maxBudget,omitempty"`
	// BudgetDuration resets the budget after this duration.
	// +optional
	BudgetDuration string `json:"budgetDuration,omitempty"`
	// TPMLimit caps tokens per minute for the team.
	// +optional
	TPMLimit *int64 `json:"tpmLimit,omitempty"`
	// RPMLimit caps requests per minute for the team.
	// +optional
	RPMLimit *int64 `json:"rpmLimit,omitempty"`
	// Metadata stores additional string metadata on the team.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
	// Blocked prevents all calls made with keys in this team.
	// +optional
	Blocked *bool `json:"blocked,omitempty"`
}

type LiteLLMTeamStatus struct {
	// TeamID is the remote LiteLLM team ID.
	// +optional
	TeamID string `json:"teamID,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=llteam
// +kubebuilder:printcolumn:name="Team ID",type=string,JSONPath=`.status.teamID`
// +kubebuilder:printcolumn:name="Proxy",type=string,JSONPath=`.spec.proxyRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type LiteLLMTeam struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LiteLLMTeamSpec   `json:"spec,omitempty"`
	Status            LiteLLMTeamStatus `json:"status,omitempty"`
}

func (t *LiteLLMTeam) DesiredTeamID() string {
	if t.Spec.TeamID != "" {
		return t.Spec.TeamID
	}
	return t.Name
}

// +kubebuilder:object:root=true
type LiteLLMTeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiteLLMTeam `json:"items"`
}

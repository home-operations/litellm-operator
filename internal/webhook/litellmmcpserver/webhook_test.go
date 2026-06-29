package litellmmcpserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

func mcpServer(spec litellmv1alpha1.LiteLLMMCPServerSpec) *litellmv1alpha1.LiteLLMMCPServer {
	return &litellmv1alpha1.LiteLLMMCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: "srv", Namespace: "ai"},
		Spec:       spec,
	}
}

func TestValidate_ExternalURLIsValid(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), mcpServer(litellmv1alpha1.LiteLLMMCPServerSpec{
		URL: "https://api.githubcopilot.com/mcp",
	}))
	require.NoError(t, err)
}

func TestValidate_WorkloadIsValid(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), mcpServer(litellmv1alpha1.LiteLLMMCPServerSpec{
		Workload: &litellmv1alpha1.MCPWorkloadSpec{Image: "mcp/grafana:latest"},
	}))
	require.NoError(t, err)
}

func TestValidate_RejectsURLAndWorkloadTogether(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateUpdate(context.Background(), nil, mcpServer(litellmv1alpha1.LiteLLMMCPServerSpec{
		URL:      "https://example.com/mcp",
		Workload: &litellmv1alpha1.MCPWorkloadSpec{Image: "mcp/grafana:latest"},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_RejectsNeitherURLNorWorkload(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), mcpServer(litellmv1alpha1.LiteLLMMCPServerSpec{
		Transport: "http",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

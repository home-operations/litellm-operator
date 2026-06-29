package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

func mcpWorkloadServer(name string, w litellmv1alpha1.MCPWorkloadSpec) *litellmv1alpha1.LiteLLMMCPServer {
	return &litellmv1alpha1.LiteLLMMCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ai"},
		Spec:       litellmv1alpha1.LiteLLMMCPServerSpec{Workload: &w},
	}
}

func TestBuildMCPDeployment_CarriesWorkloadFields(t *testing.T) {
	replicas := int32(2)
	s := mcpWorkloadServer("grafana", litellmv1alpha1.MCPWorkloadSpec{
		Image:    "mcp/grafana:latest",
		Replicas: &replicas,
		Args:     []string{"-t", "streamable-http", "--address", "0.0.0.0:8000"},
		Port:     8000,
		Env:      []corev1.EnvVar{{Name: "GRAFANA_URL", Value: "http://grafana:3000"}},
	})

	deploy := buildMCPDeployment(s)
	assert.Equal(t, "grafana", deploy.Name)
	assert.Equal(t, "ai", deploy.Namespace)
	require.NotNil(t, deploy.Spec.Replicas)
	assert.Equal(t, int32(2), *deploy.Spec.Replicas)

	c := deploy.Spec.Template.Spec.Containers[0]
	assert.Equal(t, mcpContainer, c.Name)
	assert.Equal(t, "mcp/grafana:latest", c.Image)
	assert.Equal(t, []string{"-t", "streamable-http", "--address", "0.0.0.0:8000"}, c.Args)
	require.Len(t, c.Ports, 1)
	assert.Equal(t, int32(8000), c.Ports[0].ContainerPort)
	assert.Equal(t, "GRAFANA_URL", c.Env[0].Name)

	// Selector and pod template labels must match so the Service finds the pod.
	assert.Equal(t, deploy.Spec.Selector.MatchLabels, deploy.Spec.Template.Labels)
}

func TestBuildMCPService_TargetsWorkloadPort(t *testing.T) {
	s := mcpWorkloadServer("victoria-logs", litellmv1alpha1.MCPWorkloadSpec{
		Image: "vl:latest",
		Port:  8081,
	})
	svc := buildMCPService(s)
	assert.Equal(t, "victoria-logs", svc.Name)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	require.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, int32(8081), svc.Spec.Ports[0].Port)
	assert.Equal(t, int32(8081), svc.Spec.Ports[0].TargetPort.IntVal)
	assert.Equal(t, svc.Spec.Selector, mcpSelectorLabels(s))
}

func TestBuildMCPService_DefaultsPortWhenUnset(t *testing.T) {
	s := mcpWorkloadServer("ha-mcp", litellmv1alpha1.MCPWorkloadSpec{Image: "ha:latest"})
	svc := buildMCPService(s)
	assert.Equal(t, int32(8080), svc.Spec.Ports[0].Port)
}

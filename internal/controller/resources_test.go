package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const proxyName = "main"

func TestBuildDeployment_DefaultProbesHitLiteLLMHealthEndpoints(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"}}
	d := buildDeployment(proxy, "hash", nil)
	c := d.Spec.Template.Spec.Containers[0]

	require.NotNil(t, c.LivenessProbe)
	require.NotNil(t, c.LivenessProbe.HTTPGet)
	assert.Equal(t, "/health/liveliness", c.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(proxyPort), c.LivenessProbe.HTTPGet.Port.IntVal)

	require.NotNil(t, c.ReadinessProbe)
	require.NotNil(t, c.ReadinessProbe.HTTPGet)
	assert.Equal(t, "/health/readiness", c.ReadinessProbe.HTTPGet.Path)
}

func TestBuildDeployment_ProbeOverrideWins(t *testing.T) {
	custom := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: "/custom"},
		},
	}
	proxy := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec:       litellmv1alpha1.LiteLLMProxySpec{LivenessProbe: custom},
	}
	d := buildDeployment(proxy, "hash", nil)
	c := d.Spec.Template.Spec.Containers[0]

	assert.Equal(t, "/custom", c.LivenessProbe.HTTPGet.Path)
	// readiness still defaulted
	assert.Equal(t, "/health/readiness", c.ReadinessProbe.HTTPGet.Path)
}

func TestBuildService_DefaultsPortWhenUnset(t *testing.T) {
	// A proxy with no service block (e.g. minimal api-mode) must still get a valid port.
	proxy := &litellmv1alpha1.LiteLLMProxy{ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"}}
	svc := buildService(proxy)
	assert.Equal(t, int32(proxyPort), svc.Spec.Ports[0].Port)

	proxy.Spec.Service.Port = 8080
	assert.Equal(t, int32(8080), buildService(proxy).Spec.Ports[0].Port)
}

func TestBuildDeployment_ConfigHashOnPodTemplate(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"}}
	d := buildDeployment(proxy, "abc123", nil)
	assert.Equal(t, "abc123", d.Spec.Template.Annotations[configHashAnnotation])
}

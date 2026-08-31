package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	proxyName          = "main"
	proxyRouteHostname = "llm.example.com"
	gatewayName        = "gateway"
)

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

func TestBuildRoute_PassesFiltersToGeneratedRule(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec: litellmv1alpha1.LiteLLMProxySpec{Route: &litellmv1alpha1.ProxyRoute{
			Hostnames:  []string{proxyRouteHostname},
			ParentRefs: []gatewayv1.ParentReference{{Name: gatewayName}},
			Filters: []runtime.RawExtension{{Raw: mustJSON(t, gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Set: []gatewayv1.HTTPHeader{{Name: "x-litellm-session-id", Value: "%REQ(x-session-id)%"}},
				},
			})}},
		}},
	}

	route := buildRoute(proxy)
	require.Len(t, route.Spec.Rules, 1)
	require.Len(t, route.Spec.Rules[0].Filters, 1)
	assert.Equal(t, gatewayv1.HTTPRouteFilterRequestHeaderModifier, route.Spec.Rules[0].Filters[0].Type)
}

func TestBuildRoute_PassesParentRefsVerbatim(t *testing.T) {
	parentRefs := []gatewayv1.ParentReference{
		{Name: gatewayName},
		{
			Group:       new(gatewayv1.Group("gateway.networking.k8s.io")),
			Kind:        new(gatewayv1.Kind("ListenerSet")),
			Namespace:   new(gatewayv1.Namespace("network")),
			Name:        "internal",
			SectionName: new(gatewayv1.SectionName("https")),
		},
		{
			Group: new(gatewayv1.Group("")),
			Kind:  new(gatewayv1.Kind("Service")),
			Name:  "service",
			Port:  new(gatewayv1.PortNumber(8080)),
		},
	}
	proxy := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec: litellmv1alpha1.LiteLLMProxySpec{
			Route: &litellmv1alpha1.ProxyRoute{
				Hostnames:  []string{proxyRouteHostname},
				ParentRefs: parentRefs,
			},
		},
	}

	route := buildRoute(proxy)
	require.Len(t, route.Spec.ParentRefs, 3)
	assert.Equal(t, parentRefs, route.Spec.ParentRefs)

	// An unset group/kind/port must stay unset so the apiserver applies the
	// HTTPRoute defaults rather than the operator emitting group: "".
	assert.Nil(t, route.Spec.ParentRefs[0].Group)
	assert.Nil(t, route.Spec.ParentRefs[0].Kind)
	assert.Nil(t, route.Spec.ParentRefs[0].Port)
	require.NotNil(t, route.Spec.ParentRefs[2].Port)
	assert.Equal(t, gatewayv1.PortNumber(8080), *route.Spec.ParentRefs[2].Port)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func TestBuildRoute_OmitsFiltersWhenUnset(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec: litellmv1alpha1.LiteLLMProxySpec{Route: &litellmv1alpha1.ProxyRoute{
			Hostnames:  []string{proxyRouteHostname},
			ParentRefs: []gatewayv1.ParentReference{{Name: gatewayName}},
		}},
	}

	route := buildRoute(proxy)
	assert.Nil(t, route.Spec.Rules[0].Filters)
}

func TestBuildDeployment_ConfigHashOnPodTemplate(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"}}
	d := buildDeployment(proxy, "abc123", nil)
	assert.Equal(t, "abc123", d.Spec.Template.Annotations[configHashAnnotation])
}

func TestBuildDeployment_MergesUserVolumesAfterConfig(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec: litellmv1alpha1.LiteLLMProxySpec{
			Volumes: []corev1.Volume{{
				Name: "chatgpt-tokens",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: appName},
				},
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "chatgpt-tokens",
				MountPath: "/app/chatgpt_tokens",
			}},
		},
	}
	d := buildDeployment(proxy, "hash", nil)
	pod := d.Spec.Template.Spec

	// The operator-managed config volume/mount stays first and is not shadowed.
	require.Len(t, pod.Volumes, 2)
	assert.Equal(t, configVolumeName, pod.Volumes[0].Name)
	require.NotNil(t, pod.Volumes[0].ConfigMap)
	assert.Equal(t, "chatgpt-tokens", pod.Volumes[1].Name)
	require.NotNil(t, pod.Volumes[1].PersistentVolumeClaim)
	assert.Equal(t, appName, pod.Volumes[1].PersistentVolumeClaim.ClaimName)

	mounts := pod.Containers[0].VolumeMounts
	require.Len(t, mounts, 2)
	assert.Equal(t, configVolumeName, mounts[0].Name)
	assert.Equal(t, configMountPath, mounts[0].MountPath)
	assert.Equal(t, "chatgpt-tokens", mounts[1].Name)
	assert.Equal(t, "/app/chatgpt_tokens", mounts[1].MountPath)
}

func TestBuildDeployment_PodAnnotationsMergeButConfigHashWins(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec: litellmv1alpha1.LiteLLMProxySpec{
			PodAnnotations: map[string]string{
				"reloader.stakater.com/auto": "enabled",
				configHashAnnotation:         "override-attempt",
			},
		},
	}
	d := buildDeployment(proxy, "realhash", nil)
	assert.Equal(t, "enabled", d.Spec.Template.Annotations["reloader.stakater.com/auto"])
	assert.Equal(t, "realhash", d.Spec.Template.Annotations[configHashAnnotation])
}

func TestBuildDeployment_PodLabelsCannotOverrideSelector(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"}}
	selectorKey := ""
	for k := range selectorLabels(proxy) {
		selectorKey = k
		break
	}
	proxy.Spec.PodLabels = map[string]string{selectorKey: "hijacked", "team": "ai"}

	d := buildDeployment(proxy, "hash", nil)
	assert.Equal(t, selectorLabels(proxy)[selectorKey], d.Spec.Template.Labels[selectorKey])
	assert.Equal(t, "ai", d.Spec.Template.Labels["team"])
}

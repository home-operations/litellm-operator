package litellmproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	proxyName     = "main"
	routeHostname = "litellm.example.com"
)

func proxyWith(route *litellmv1alpha1.ProxyRoute) *litellmv1alpha1.LiteLLMProxy {
	return &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec:       litellmv1alpha1.LiteLLMProxySpec{Route: route},
	}
}

func TestValidate_APIModeRequiresMasterKeyRef(t *testing.T) {
	v := &Validator{}
	p := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: "ai"},
		Spec:       litellmv1alpha1.LiteLLMProxySpec{ApplyMode: "api"},
	}
	_, err := v.ValidateCreate(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiAccess.masterKeyRef")

	p.Spec.APIAccess = &litellmv1alpha1.APIAccessSpec{MasterKeyRef: litellmv1alpha1.SecretKeyRef{Name: "litellm", Key: "MASTER_KEY"}}
	_, err = v.ValidateCreate(context.Background(), p)
	require.NoError(t, err)
}

func TestValidate_NoRouteIsValid(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), proxyWith(nil))
	require.NoError(t, err)
}

func TestValidate_ValidRoute(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), proxyWith(&litellmv1alpha1.ProxyRoute{
		Hostnames:  []string{routeHostname},
		ParentRefs: []gatewayv1.ParentReference{{Name: "envoy-external", Namespace: new(gatewayv1.Namespace("network"))}},
	}))
	require.NoError(t, err)
}

func TestValidate_RejectsRouteWithoutHostnames(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), proxyWith(&litellmv1alpha1.ProxyRoute{
		ParentRefs: []gatewayv1.ParentReference{{Name: "envoy-external"}},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hostnames must not be empty")
}

func TestValidate_RejectsParentRefWithoutName(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateUpdate(context.Background(), nil, proxyWith(&litellmv1alpha1.ProxyRoute{
		Hostnames:  []string{routeHostname},
		ParentRefs: []gatewayv1.ParentReference{{Name: ""}},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

func TestValidate_ParentRefs(t *testing.T) {
	gw := gatewayv1.Group(gatewayv1.GroupName)
	core := gatewayv1.Group("")
	service := gatewayv1.Kind("Service")
	https := gatewayv1.SectionName("https")
	http := gatewayv1.SectionName("http")

	tests := []struct {
		name    string
		refs    []gatewayv1.ParentReference
		wantErr string
	}{
		{
			name: "service in core group",
			refs: []gatewayv1.ParentReference{{Group: &core, Kind: &service, Name: proxyName}},
		},
		{
			name:    "service without group",
			refs:    []gatewayv1.ParentReference{{Kind: &service, Name: proxyName}},
			wantErr: `parentRefs[0].group must be explicitly set to ""`,
		},
		{
			name:    "service with defaulted gateway group",
			refs:    []gatewayv1.ParentReference{{Group: &gw, Kind: &service, Name: proxyName}},
			wantErr: `parentRefs[0].group must be explicitly set to ""`,
		},
		{
			name: "same parent with distinct section names",
			refs: []gatewayv1.ParentReference{{Name: "gw", SectionName: &https}, {Name: "gw", SectionName: &http}},
		},
		{
			name: "same name in different namespaces",
			refs: []gatewayv1.ParentReference{{Name: "gw"}, {Name: "gw", Namespace: new(gatewayv1.Namespace("network"))}},
		},
		{
			name:    "same parent differing only by port",
			refs:    []gatewayv1.ParentReference{{Name: "gw", Port: new(gatewayv1.PortNumber(80))}, {Name: "gw", Port: new(gatewayv1.PortNumber(443))}},
			wantErr: "parentRefs[0] and [1] reference the same parent; each must set a distinct sectionName",
		},
		{
			name:    "same parent with one section name",
			refs:    []gatewayv1.ParentReference{{Name: "gw", SectionName: &https}, {Name: "gw"}},
			wantErr: "parentRefs[0] and [1] reference the same parent; each must set a distinct sectionName",
		},
		{
			name:    "same parent and section name",
			refs:    []gatewayv1.ParentReference{{Name: "gw", SectionName: &https}, {Group: &gw, Name: "gw", SectionName: &https}},
			wantErr: "parentRefs[0] and [1] reference the same parent and sectionName",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{}
			_, err := v.ValidateCreate(context.Background(), proxyWith(&litellmv1alpha1.ProxyRoute{
				Hostnames:  []string{routeHostname},
				ParentRefs: tt.refs,
			}))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidate_RejectsReservedConfigVolumeName(t *testing.T) {
	v := &Validator{}
	p := proxyWith(nil)
	p.Spec.Volumes = []corev1.Volume{{Name: litellmv1alpha1.ProxyConfigVolumeName}}
	_, err := v.ValidateCreate(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestValidate_RejectsReservedVolumeMountName(t *testing.T) {
	v := &Validator{}
	p := proxyWith(nil)
	p.Spec.VolumeMounts = []corev1.VolumeMount{{Name: litellmv1alpha1.ProxyConfigVolumeName, MountPath: "/somewhere"}}
	_, err := v.ValidateCreate(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestValidate_RejectsReservedMountPath(t *testing.T) {
	v := &Validator{}
	p := proxyWith(nil)
	p.Spec.VolumeMounts = []corev1.VolumeMount{{Name: "shadow", MountPath: litellmv1alpha1.ProxyConfigMountPath}}
	_, err := v.ValidateCreate(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestValidate_AllowsUserVolumes(t *testing.T) {
	v := &Validator{}
	p := proxyWith(nil)
	p.Spec.Volumes = []corev1.Volume{{
		Name: "chatgpt-tokens",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "litellm"},
		},
	}}
	p.Spec.VolumeMounts = []corev1.VolumeMount{{Name: "chatgpt-tokens", MountPath: "/app/chatgpt_tokens"}}
	_, err := v.ValidateCreate(context.Background(), p)
	require.NoError(t, err)
}

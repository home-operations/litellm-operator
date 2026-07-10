package litellmproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

func proxyWith(route *litellmv1alpha1.ProxyRoute) *litellmv1alpha1.LiteLLMProxy {
	return &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "ai"},
		Spec:       litellmv1alpha1.LiteLLMProxySpec{Route: route},
	}
}

func TestValidate_APIModeRequiresMasterKeyRef(t *testing.T) {
	v := &Validator{}
	p := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "ai"},
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
		Hostnames:  []string{"litellm.example.com"},
		ParentRefs: []litellmv1alpha1.RouteParentRef{{Name: "envoy-external", Namespace: "network"}},
	}))
	require.NoError(t, err)
}

func TestValidate_RejectsRouteWithoutHostnames(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), proxyWith(&litellmv1alpha1.ProxyRoute{
		ParentRefs: []litellmv1alpha1.RouteParentRef{{Name: "envoy-external"}},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hostnames must not be empty")
}

func TestValidate_RejectsParentRefWithoutName(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateUpdate(context.Background(), nil, proxyWith(&litellmv1alpha1.ProxyRoute{
		Hostnames:  []string{"litellm.example.com"},
		ParentRefs: []litellmv1alpha1.RouteParentRef{{Name: ""}},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
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

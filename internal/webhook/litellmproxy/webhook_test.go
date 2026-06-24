package litellmproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

func proxyWith(route *litellmv1alpha1.ProxyRoute) *litellmv1alpha1.LiteLLMProxy {
	return &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "ai"},
		Spec:       litellmv1alpha1.LiteLLMProxySpec{Route: route},
	}
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

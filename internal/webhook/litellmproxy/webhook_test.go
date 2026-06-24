package litellmproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

func proxyWith(sel metav1.LabelSelector) *litellmv1alpha1.LiteLLMProxy {
	return &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "ai"},
		Spec:       litellmv1alpha1.LiteLLMProxySpec{ModelSelector: sel},
	}
}

func TestValidate_RejectsEmptySelector(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), proxyWith(metav1.LabelSelector{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modelSelector must not be empty")
}

func TestValidate_AllowsMatchLabels(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), proxyWith(metav1.LabelSelector{
		MatchLabels: map[string]string{"proxy": "main"},
	}))
	require.NoError(t, err)
}

func TestValidate_AllowsMatchExpressions(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateUpdate(context.Background(), nil, proxyWith(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "proxy",
			Operator: metav1.LabelSelectorOpExists,
		}},
	}))
	require.NoError(t, err)
}

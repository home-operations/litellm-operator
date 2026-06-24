package litellmguardrail

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	guardrailAporia = "aporia"
	guardrailName   = "aporia-pre"
)

func guardrail(spec litellmv1alpha1.LiteLLMGuardrailSpec) *litellmv1alpha1.LiteLLMGuardrail {
	return &litellmv1alpha1.LiteLLMGuardrail{
		ObjectMeta: metav1.ObjectMeta{Name: "g", Namespace: "ai"},
		Spec:       spec,
	}
}

func TestValidate_Valid(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), guardrail(litellmv1alpha1.LiteLLMGuardrailSpec{
		GuardrailName: guardrailName, Guardrail: guardrailAporia,
		APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
	}))
	require.NoError(t, err)
}

func TestValidate_RejectsBothAPIKeyAndRef(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateCreate(context.Background(), guardrail(litellmv1alpha1.LiteLLMGuardrailSpec{
		GuardrailName: guardrailName, Guardrail: guardrailAporia,
		APIKey:    "os.environ/FOO",
		APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_RejectsBothAPIBaseAndRef(t *testing.T) {
	v := &Validator{}
	_, err := v.ValidateUpdate(context.Background(), nil, guardrail(litellmv1alpha1.LiteLLMGuardrailSpec{
		GuardrailName: guardrailName, Guardrail: guardrailAporia,
		APIBase:    "https://x",
		APIBaseRef: &litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

package litellmmodel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

func scheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, litellmv1alpha1.AddToScheme(s))
	return s
}

func modelWithRef(name, secret string) *litellmv1alpha1.LiteLLMModel {
	return &litellmv1alpha1.LiteLLMModel{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ai"},
		Spec: litellmv1alpha1.LiteLLMModelSpec{
			ModelName: name,
			Params: litellmv1alpha1.LiteLLMParams{
				Model:     "openai/" + name,
				APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: secret, Key: "key"},
			},
		},
	}
}

func TestValidate_RejectsBothAPIKeyAndRef(t *testing.T) {
	m := modelWithRef("glm", "s")
	m.Spec.Params.APIKey = "os.environ/FOO"
	v := &Validator{Client: fake.NewClientBuilder().WithScheme(scheme(t)).Build()}

	_, err := v.ValidateCreate(context.Background(), m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_AllowsLiteralAPIKeyOnly(t *testing.T) {
	m := &litellmv1alpha1.LiteLLMModel{
		ObjectMeta: metav1.ObjectMeta{Name: "glm", Namespace: "ai"},
		Spec: litellmv1alpha1.LiteLLMModelSpec{
			ModelName: "glm",
			Params:    litellmv1alpha1.LiteLLMParams{Model: "openai/glm", APIKey: "os.environ/FOO"},
		},
	}
	v := &Validator{Client: fake.NewClientBuilder().WithScheme(scheme(t)).Build()}

	warnings, err := v.ValidateCreate(context.Background(), m)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidate_RejectsEnvVarCollision(t *testing.T) {
	existing := modelWithRef("minimax-m3", "secret-a")
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(existing).Build()
	v := &Validator{Client: c}

	// Different resource name, same sanitized env var (dot vs dash).
	incoming := modelWithRef("minimax.m3", "secret-b")
	_, err := v.ValidateCreate(context.Background(), incoming)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same API key env var")
	assert.Contains(t, err.Error(), "minimax-m3")
}

func TestValidate_AllowsDistinctEnvVars(t *testing.T) {
	existing := modelWithRef("minimax-m3", "secret-a")
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(existing).Build()
	v := &Validator{Client: c}

	incoming := modelWithRef("minimax-m2", "secret-b")
	_, err := v.ValidateCreate(context.Background(), incoming)
	require.NoError(t, err)
}

func TestValidate_UpdateSameNameNoSelfCollision(t *testing.T) {
	existing := modelWithRef("glm", "secret-a")
	c := fake.NewClientBuilder().WithScheme(scheme(t)).WithObjects(existing).Build()
	v := &Validator{Client: c}

	updated := modelWithRef("glm", "secret-a")
	updated.Spec.Params.Model = "openai/glm-next"
	_, err := v.ValidateUpdate(context.Background(), existing, updated)
	require.NoError(t, err)
}

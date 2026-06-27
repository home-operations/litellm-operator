package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
)

const chatEndpoint = "http://h:8080/v1/chat/completions"

func readyISVC(name, modelRef, endpoint string) *inferencev1alpha1.InferenceService {
	isvc := &inferencev1alpha1.InferenceService{}
	isvc.Name = name
	isvc.Namespace = "ai"
	isvc.Spec.ModelRef = modelRef
	isvc.Status.Phase = inferenceReadyPhase
	isvc.Status.Endpoint = endpoint
	return isvc
}

func modelWithContext(name string, ctxLen uint64) *inferencev1alpha1.Model {
	m := &inferencev1alpha1.Model{}
	m.Name = name
	if ctxLen > 0 {
		m.Status.GGUF = &inferencev1alpha1.GGUFMetadata{ContextLength: ctxLen}
	}
	return m
}

func TestProjectInferenceService_CoreMapping(t *testing.T) {
	isvc := readyISVC("llama3", "llama3-8b",
		"http://llama3.ai.svc.cluster.local:8080/v1/chat/completions")

	got := projectInferenceService(isvc, nil)

	require.NotNil(t, got)
	assert.Equal(t, "llama3", got.Name)
	assert.Equal(t, "ai", got.Namespace)
	assert.Equal(t, "llama3", got.Spec.ModelName, "clients call the proxy by the InferenceService name")
	assert.Equal(t, "openai/llama3-8b", got.Spec.Params.Model, "provider id uses the OpenAI-compatible prefix + modelRef")
	assert.Equal(t, "http://llama3.ai.svc.cluster.local:8080/v1", got.Spec.Params.APIBase,
		"the /chat/completions suffix is trimmed so litellm appends it itself")
	assert.NotEmpty(t, got.Spec.Params.APIKey, "openai provider needs a non-empty key even for an unauthenticated endpoint")
	assert.Equal(t, managedByLLMKube, got.Labels[managedByLabel])
	assert.Equal(t, "llama3", got.Labels[llmkubeServiceLabel])
}

func TestProjectInferenceService_NoModelOmitsInfo(t *testing.T) {
	isvc := readyISVC("m", "m-ref", chatEndpoint)

	got := projectInferenceService(isvc, nil)

	assert.Nil(t, got.Spec.Info, "without a Model there is no truthful capability metadata to set")
}

func TestProjectInferenceService_ContextLengthFromGGUF(t *testing.T) {
	isvc := readyISVC("m", "m-ref", chatEndpoint)
	model := modelWithContext("m-ref", 131072)

	got := projectInferenceService(isvc, model)

	require.NotNil(t, got.Spec.Info)
	require.NotNil(t, got.Spec.Info.MaxInputTokens)
	assert.Equal(t, int64(131072), *got.Spec.Info.MaxInputTokens)
	assert.Nil(t, got.Spec.Info.SupportsFunctionCalling, "capabilities LLMKube cannot report are left unset, not asserted")
	assert.Nil(t, got.Spec.Info.SupportsVision)
}

func TestProjectInferenceService_GGUFWithoutContextOmitsInfo(t *testing.T) {
	isvc := readyISVC("m", "m-ref", chatEndpoint)
	model := modelWithContext("m-ref", 0)

	got := projectInferenceService(isvc, model)

	assert.Nil(t, got.Spec.Info)
}

func TestLLMKubeAPIBase(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"standard chat path", chatEndpoint, "http://h:8080/v1"},
		{"non-standard path passes through", "http://h:8080/custom", "http://h:8080/custom"},
		{"empty endpoint", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, llmkubeAPIBase(tt.endpoint))
		})
	}
}

func TestInferenceServiceReady(t *testing.T) {
	tests := []struct {
		name  string
		phase string
		endpt string
		want  bool
	}{
		{"ready with endpoint", inferenceReadyPhase, chatEndpoint, true},
		{"ready but endpoint not yet populated", inferenceReadyPhase, "", false},
		{"progressing", "Progressing", chatEndpoint, false},
		{"failed", "Failed", "", false},
		{"empty phase", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isvc := &inferencev1alpha1.InferenceService{}
			isvc.Status.Phase = tt.phase
			isvc.Status.Endpoint = tt.endpt
			assert.Equal(t, tt.want, inferenceServiceReady(isvc))
		})
	}
}

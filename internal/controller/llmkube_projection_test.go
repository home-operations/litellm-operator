package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
)

const (
	chatEndpoint   = "http://h:8080/v1/chat/completions"
	trimmedAPIBase = "http://h:8080/v1"
)

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

func TestLLMKubeModelMode(t *testing.T) {
	withArgs := func(args ...string) *inferencev1alpha1.InferenceService {
		isvc := &inferencev1alpha1.InferenceService{}
		isvc.Spec.ExtraArgs = args
		return isvc
	}

	t.Run("chat model has no mode", func(t *testing.T) {
		assert.Equal(t, "", llmkubeModelMode(&inferencev1alpha1.InferenceService{}))
	})
	t.Run("embedding from --embedding flag", func(t *testing.T) {
		assert.Equal(t, "embedding", llmkubeModelMode(withArgs("--embedding", "--pooling", "last")))
	})
	t.Run("rerank wins over --embedding when both are present", func(t *testing.T) {
		assert.Equal(t, "rerank", llmkubeModelMode(withArgs("--reranking", "--pooling", "rank", "--embedding")))
	})
	t.Run("inferred from Args (generic runtime)", func(t *testing.T) {
		isvc := &inferencev1alpha1.InferenceService{}
		isvc.Spec.Args = []string{"--embeddings"}
		assert.Equal(t, "embedding", llmkubeModelMode(isvc))
	})
	t.Run("inferred from endpoint path", func(t *testing.T) {
		isvc := &inferencev1alpha1.InferenceService{}
		isvc.Spec.Endpoint = &inferencev1alpha1.EndpointSpec{Path: "/v1/rerank"}
		assert.Equal(t, "rerank", llmkubeModelMode(isvc))
	})
	t.Run("annotation overrides the heuristic", func(t *testing.T) {
		isvc := withArgs("--embedding")
		isvc.Annotations = map[string]string{llmkubeModeAnnotation: "rerank"}
		assert.Equal(t, "rerank", llmkubeModelMode(isvc))
	})
}

func TestProjectInferenceService_EmbeddingMode(t *testing.T) {
	// LLMKube reports the endpoint at the operation-specific path for
	// embedding services, not the chat-completions default.
	isvc := readyISVC("embed", "embed-ref", "http://embed.ai.svc.cluster.local:8080/v1/embeddings")
	isvc.Spec.ExtraArgs = []string{"--embedding", "--pooling", "last"}

	got := projectInferenceService(isvc, nil)

	require.NotNil(t, got.Spec.Info)
	assert.Equal(t, "embedding", got.Spec.Info.Mode)
	assert.Nil(t, got.Spec.Info.MaxInputTokens, "no Model means no context length, but mode still sets info")
	assert.Equal(t, "http://embed.ai.svc.cluster.local:8080/v1", got.Spec.Params.APIBase,
		"the /embeddings suffix must be trimmed too, or litellm double-appends it")
}

func TestLLMKubeAPIBase(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"standard chat path", chatEndpoint, trimmedAPIBase},
		{"embedding path", "http://h:8080/v1/embeddings", trimmedAPIBase},
		{"rerank path", "http://h:8080/v1/rerank", trimmedAPIBase},
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

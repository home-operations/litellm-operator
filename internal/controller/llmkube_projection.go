package controller

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	// managedByLabel marks LiteLLMModels the operator generates from an LLMKube
	// InferenceService. A proxy's modelSelector can target it, and the
	// reconciler uses it to tell its own models apart from hand-written ones.
	managedByLabel   = "litellm.home-operations.com/managed-by"
	managedByLLMKube = "llmkube"

	// llmkubeServiceLabel records the source InferenceService name.
	llmkubeServiceLabel = "inference.llmkube.dev/service"

	// llmkubeDummyAPIKey is a non-empty placeholder. LLMKube serves an
	// unauthenticated OpenAI-compatible endpoint, but litellm's openai provider
	// errors when api_key is unset (it falls back to OPENAI_API_KEY). Any
	// non-empty string satisfies it; the upstream ignores the value.
	llmkubeDummyAPIKey = "sk-llmkube-noauth"

	openAICompatPrefix    = "openai/"
	chatCompletionsSuffix = "/chat/completions"
	inferenceReadyPhase   = "Ready"
)

// projectInferenceService renders the LiteLLMModel that mirrors a Ready LLMKube
// InferenceService. It is a pure function: no client calls, no ownership wiring
// (the reconciler sets the controller reference). model may be nil when the
// referenced Model is unavailable; capability metadata is then omitted rather
// than guessed.
//
// The model is keyed and named deterministically by the InferenceService name,
// so a client calls the proxy with that name. The provider id (litellm_params.model)
// uses the InferenceService's resolved model reference, which matches how LLMKube
// derives the OpenAI "model" string for its own gateway.
func projectInferenceService(isvc *inferencev1alpha1.InferenceService, model *inferencev1alpha1.Model) *litellmv1alpha1.LiteLLMModel {
	out := &litellmv1alpha1.LiteLLMModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      isvc.Name,
			Namespace: isvc.Namespace,
			Labels: map[string]string{
				managedByLabel:      managedByLLMKube,
				llmkubeServiceLabel: isvc.Name,
			},
		},
		Spec: litellmv1alpha1.LiteLLMModelSpec{
			ModelName: isvc.Name,
			Params: litellmv1alpha1.LiteLLMParams{
				Model:   openAICompatPrefix + isvc.Spec.ModelRef,
				APIBase: llmkubeAPIBase(isvc.Status.Endpoint),
				APIKey:  llmkubeDummyAPIKey,
			},
		},
	}

	if info := projectModelInfo(model); info != nil {
		out.Spec.Info = info
	}
	return out
}

// llmkubeAPIBase turns an InferenceService status endpoint into a litellm
// api_base. LLMKube reports the full chat URL (…/v1/chat/completions); litellm's
// openai provider wants the server root (…/v1) and appends /chat/completions
// itself, so the suffix is trimmed. A non-standard path is passed through
// unchanged.
func llmkubeAPIBase(endpoint string) string {
	return strings.TrimSuffix(endpoint, chatCompletionsSuffix)
}

// projectModelInfo fills only the capability fields LLMKube can report
// truthfully. Context length comes from the parsed GGUF header; everything else
// (function calling, vision, prompt caching) is left unset rather than asserted.
func projectModelInfo(model *inferencev1alpha1.Model) *litellmv1alpha1.ModelInfo {
	if model == nil || model.Status.GGUF == nil {
		return nil
	}
	ctxLen := model.Status.GGUF.ContextLength
	if ctxLen == 0 {
		return nil
	}
	maxInput := int64(ctxLen)
	return &litellmv1alpha1.ModelInfo{MaxInputTokens: &maxInput}
}

// inferenceServiceReady reports whether the InferenceService is serving traffic
// at a known endpoint. Phase alone is not enough: the endpoint can lag the Ready
// phase by a reconcile.
func inferenceServiceReady(isvc *inferencev1alpha1.InferenceService) bool {
	return isvc.Status.Phase == inferenceReadyPhase && isvc.Status.Endpoint != ""
}

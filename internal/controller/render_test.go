package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const modelOpenAIM = "openai/m"

func raw(s string) *runtime.RawExtension { return &runtime.RawExtension{Raw: []byte(s)} }

func model(name, modelName string, params litellmv1alpha1.LiteLLMParams) litellmv1alpha1.LiteLLMModel {
	return litellmv1alpha1.LiteLLMModel{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       litellmv1alpha1.LiteLLMModelSpec{ModelName: modelName, Params: params},
	}
}

// parse re-parses rendered YAML back into a generic structure for assertions.
func parse(t *testing.T, y string) map[string]any {
	t.Helper()
	out := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(y), &out))
	return out
}

func TestRenderConfig_SecretKeyBecomesEnvRefNotInlineSecret(t *testing.T) {
	m := model("glm", "glm-5.2", litellmv1alpha1.LiteLLMParams{
		Model:     "openai/glm-5.2",
		APIBase:   "https://api.z.ai/v4",
		APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "zai", Key: "apikey"},
	})
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	// The rendered config references the env var, never the secret name or key.
	assert.Contains(t, got.yaml, "os.environ/LITELLM_MODELKEY_GLM")
	assert.NotContains(t, got.yaml, "apikey")
	assert.NotContains(t, got.yaml, "zai")

	// The secret is wired as exactly one env var pointing at the right secret/key.
	require.Len(t, got.envVars, 1)
	ev := got.envVars[0]
	assert.Equal(t, "LITELLM_MODELKEY_GLM", ev.Name)
	require.NotNil(t, ev.ValueFrom)
	require.NotNil(t, ev.ValueFrom.SecretKeyRef)
	assert.Equal(t, "zai", ev.ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "apikey", ev.ValueFrom.SecretKeyRef.Key)
	assert.Equal(t, corev1.EnvVar{
		Name: "LITELLM_MODELKEY_GLM",
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "zai"},
			Key:                  "apikey",
		}},
	}, ev)
}

func TestRenderConfig_LiteralAPIKeyNoEnvVar(t *testing.T) {
	m := model("llama", "gemma", litellmv1alpha1.LiteLLMParams{
		Model:  "openai/gemma",
		APIKey: "os.environ/LLAMA_API_KEY",
	})
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	assert.Empty(t, got.envVars)
	cfg := parse(t, got.yaml)
	list := cfg["model_list"].([]any)
	params := list[0].(map[string]any)["litellm_params"].(map[string]any)
	assert.Equal(t, "os.environ/LLAMA_API_KEY", params["api_key"])
}

func TestRenderConfig_TypedFieldsAndPassthroughMerge(t *testing.T) {
	drop := true
	m := model("qwen", "qwen", litellmv1alpha1.LiteLLMParams{
		Model:      "openai/qwen3.6-27b",
		APIBase:    "https://super",
		APIVersion: "2024-01-01",
		DropParams: &drop,
		RPM:        ptr[int64](100),
		TPM:        ptr[int64](200000),
		Additional: raw(`{"timeout": 30, "stream_timeout": 10}`),
	})
	m.Spec.Info = raw(`{"max_input_tokens": 262144, "supports_function_calling": true}`)

	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	entry := cfg["model_list"].([]any)[0].(map[string]any)
	params := entry["litellm_params"].(map[string]any)

	assert.Equal(t, "openai/qwen3.6-27b", params["model"])
	assert.Equal(t, "https://super", params["api_base"])
	assert.Equal(t, "2024-01-01", params["api_version"])
	assert.Equal(t, true, params["drop_params"])
	assert.EqualValues(t, 100, params["rpm"])
	assert.EqualValues(t, 200000, params["tpm"])
	// passthrough fields survive
	assert.EqualValues(t, 30, params["timeout"])
	assert.EqualValues(t, 10, params["stream_timeout"])

	info := entry["model_info"].(map[string]any)
	assert.EqualValues(t, 262144, info["max_input_tokens"])
	assert.Equal(t, true, info["supports_function_calling"])
}

func TestRenderConfig_TypedFieldWinsOverAdditional(t *testing.T) {
	m := model("x", "x", litellmv1alpha1.LiteLLMParams{
		Model:      "openai/real",
		Additional: raw(`{"model": "openai/should-be-overridden"}`),
	})
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)
	params := parse(t, got.yaml)["model_list"].([]any)[0].(map[string]any)["litellm_params"].(map[string]any)
	assert.Equal(t, "openai/real", params["model"])
}

func TestRenderConfig_DeterministicAcrossInputOrder(t *testing.T) {
	a := model("aaa", "a", litellmv1alpha1.LiteLLMParams{Model: "openai/a"})
	b := model("bbb", "b", litellmv1alpha1.LiteLLMParams{Model: "openai/b"})
	c := model("ccc", "c", litellmv1alpha1.LiteLLMParams{Model: "openai/c"})

	first, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{a, b, c})
	require.NoError(t, err)
	shuffled, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{c, a, b})
	require.NoError(t, err)

	assert.Equal(t, first.yaml, shuffled.yaml)
	assert.Equal(t, first.hash, shuffled.hash)

	// And the order is the sorted-by-name order.
	list := parse(t, first.yaml)["model_list"].([]any)
	assert.Equal(t, "a", list[0].(map[string]any)["model_name"])
	assert.Equal(t, "b", list[1].(map[string]any)["model_name"])
	assert.Equal(t, "c", list[2].(map[string]any)["model_name"])
}

func TestRenderConfig_HashChangesWithContent(t *testing.T) {
	base := model("m", "m", litellmv1alpha1.LiteLLMParams{Model: modelOpenAIM})
	got1, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{base})
	require.NoError(t, err)

	changed := base
	changed.Spec.Params.Model = "openai/m2"
	got2, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{changed})
	require.NoError(t, err)

	assert.NotEqual(t, got1.hash, got2.hash)
}

func TestRenderConfig_GlobalSettingsBlocks(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		Spec: litellmv1alpha1.LiteLLMProxySpec{
			GeneralSettings: raw(`{"health_check_endpoint": "/v1/health"}`),
			RouterSettings:  raw(`{"routing_strategy": "simple-shuffle"}`),
			LitellmSettings: raw(`{"cache": true}`),
		},
	}
	got, err := renderConfig(proxy, nil)
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	assert.Equal(t, "/v1/health", cfg["general_settings"].(map[string]any)["health_check_endpoint"])
	assert.Equal(t, "simple-shuffle", cfg["router_settings"].(map[string]any)["routing_strategy"])
	assert.Equal(t, true, cfg["litellm_settings"].(map[string]any)["cache"])
}

func TestRenderConfig_OmitsUnsetGlobalSettingsAndOptionalParams(t *testing.T) {
	m := model("m", "m", litellmv1alpha1.LiteLLMParams{Model: modelOpenAIM})
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	_, hasGeneral := cfg["general_settings"]
	_, hasRouter := cfg["router_settings"]
	_, hasLitellm := cfg["litellm_settings"]
	assert.False(t, hasGeneral)
	assert.False(t, hasRouter)
	assert.False(t, hasLitellm)

	params := cfg["model_list"].([]any)[0].(map[string]any)["litellm_params"].(map[string]any)
	for _, k := range []string{"api_base", "api_key", "api_version", "drop_params", "rpm", "tpm"} {
		_, ok := params[k]
		assert.Falsef(t, ok, "expected %q to be omitted when unset", k)
	}
	_, hasInfo := cfg["model_list"].([]any)[0].(map[string]any)["model_info"]
	assert.False(t, hasInfo)
}

func TestRenderConfig_InvalidJSONErrors(t *testing.T) {
	m := model("m", "m", litellmv1alpha1.LiteLLMParams{Model: modelOpenAIM, Additional: raw(`{not json`)})
	_, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "m")
}

func TestModelKeyEnvVar(t *testing.T) {
	cases := map[string]string{
		"glm":          "LITELLM_MODELKEY_GLM",
		"minimax-m3":   "LITELLM_MODELKEY_MINIMAX_M3",
		"qwen3.6-27b":  "LITELLM_MODELKEY_QWEN3_6_27B",
		"neuralwatt-x": "LITELLM_MODELKEY_NEURALWATT_X",
	}
	for in, want := range cases {
		assert.Equalf(t, want, modelKeyEnvVar(in), "input %q", in)
	}
}

func TestModelKeyEnvVar_DistinctNamesDistinctVars(t *testing.T) {
	assert.NotEqual(t, modelKeyEnvVar("minimax-m3"), modelKeyEnvVar("minimax-m2"))
}

func ptr[T any](v T) *T { return &v }

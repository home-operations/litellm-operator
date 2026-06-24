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

const (
	modelOpenAIM    = "openai/m"
	secretKeyKey    = "apikey"
	guardrailAporia = "aporia"
)

func raw(s string) *runtime.RawExtension { return &runtime.RawExtension{Raw: []byte(s)} }

// renderM renders with only models (no guardrails/MCP servers).
func renderM(proxy *litellmv1alpha1.LiteLLMProxy, models []litellmv1alpha1.LiteLLMModel) (renderedConfig, error) {
	return renderConfig(proxy, models, nil, nil)
}

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
		APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "zai", Key: secretKeyKey},
	})
	got, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
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
	got, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	assert.Empty(t, got.envVars)
	cfg := parse(t, got.yaml)
	list := cfg["model_list"].([]any)
	params := list[0].(map[string]any)[keyLitellmParams].(map[string]any)
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
	m.Spec.Info = &litellmv1alpha1.ModelInfo{
		MaxInputTokens:          ptr[int64](262144),
		SupportsFunctionCalling: ptr(true),
	}

	got, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	entry := cfg["model_list"].([]any)[0].(map[string]any)
	params := entry[keyLitellmParams].(map[string]any)

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

func TestRenderConfig_TypedModelInfoMapsToSnakeCase(t *testing.T) {
	m := model("mm", "MiniMax-M3", litellmv1alpha1.LiteLLMParams{Model: "minimax/MiniMax-M3"})
	m.Spec.Info = &litellmv1alpha1.ModelInfo{
		MaxTokens:               ptr[int64](1000000),
		MaxInputTokens:          ptr[int64](192000),
		MaxOutputTokens:         ptr[int64](16384),
		Mode:                    "messages",
		SupportsFunctionCalling: ptr(true),
		SupportsPromptCaching:   ptr(true),
		SupportsVision:          ptr(false),
		Extra:                   raw(`{"custom_key": "v"}`),
	}
	got, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	info := parse(t, got.yaml)["model_list"].([]any)[0].(map[string]any)["model_info"].(map[string]any)
	assert.EqualValues(t, 1000000, info["max_tokens"])
	assert.EqualValues(t, 192000, info["max_input_tokens"])
	assert.EqualValues(t, 16384, info["max_output_tokens"])
	assert.Equal(t, "messages", info["mode"])
	assert.Equal(t, true, info["supports_function_calling"])
	assert.Equal(t, true, info["supports_prompt_caching"])
	assert.Equal(t, false, info["supports_vision"])
	assert.Equal(t, "v", info["custom_key"])
}

func TestRenderConfig_APIBaseRefBecomesEnvRef(t *testing.T) {
	m := model("qwen", "qwen", litellmv1alpha1.LiteLLMParams{
		Model:      "openai/qwen",
		APIBaseRef: &litellmv1alpha1.SecretKeyRef{Name: "litellm", Key: "SUPER_SERVER_URL"},
		APIKeyRef:  &litellmv1alpha1.SecretKeyRef{Name: "litellm", Key: "SUPER_SERVER_PASS"},
	})
	got, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	params := parse(t, got.yaml)["model_list"].([]any)[0].(map[string]any)[keyLitellmParams].(map[string]any)
	assert.Equal(t, "os.environ/LITELLM_MODELBASE_QWEN", params["api_base"])
	assert.Equal(t, "os.environ/LITELLM_MODELKEY_QWEN", params["api_key"])

	require.Len(t, got.envVars, 2)
	byName := map[string]string{}
	for _, e := range got.envVars {
		byName[e.Name] = e.ValueFrom.SecretKeyRef.Key
	}
	assert.Equal(t, "SUPER_SERVER_URL", byName["LITELLM_MODELBASE_QWEN"])
	assert.Equal(t, "SUPER_SERVER_PASS", byName["LITELLM_MODELKEY_QWEN"])
	assert.NotContains(t, got.yaml, "SUPER_SERVER_URL")
}

func TestRenderConfig_TypedFieldWinsOverAdditional(t *testing.T) {
	m := model("x", "x", litellmv1alpha1.LiteLLMParams{
		Model:      "openai/real",
		Additional: raw(`{"model": "openai/should-be-overridden"}`),
	})
	got, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)
	params := parse(t, got.yaml)["model_list"].([]any)[0].(map[string]any)[keyLitellmParams].(map[string]any)
	assert.Equal(t, "openai/real", params["model"])
}

func TestRenderConfig_DeterministicAcrossInputOrder(t *testing.T) {
	a := model("aaa", "a", litellmv1alpha1.LiteLLMParams{Model: "openai/a"})
	b := model("bbb", "b", litellmv1alpha1.LiteLLMParams{Model: "openai/b"})
	c := model("ccc", "c", litellmv1alpha1.LiteLLMParams{Model: "openai/c"})

	first, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{a, b, c})
	require.NoError(t, err)
	shuffled, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{c, a, b})
	require.NoError(t, err)

	assert.Equal(t, first.yaml, shuffled.yaml)
	assert.Equal(t, first.hash, shuffled.hash)

	// And the order is the sorted-by-name order.
	list := parse(t, first.yaml)["model_list"].([]any)
	assert.Equal(t, "a", list[0].(map[string]any)[keyModelName])
	assert.Equal(t, "b", list[1].(map[string]any)[keyModelName])
	assert.Equal(t, "c", list[2].(map[string]any)[keyModelName])
}

func TestRenderConfig_HashChangesWithContent(t *testing.T) {
	base := model("m", "m", litellmv1alpha1.LiteLLMParams{Model: modelOpenAIM})
	got1, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{base})
	require.NoError(t, err)

	changed := base
	changed.Spec.Params.Model = "openai/m2"
	got2, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{changed})
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
	got, err := renderM(proxy, nil)
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	assert.Equal(t, "/v1/health", cfg["general_settings"].(map[string]any)["health_check_endpoint"])
	assert.Equal(t, "simple-shuffle", cfg["router_settings"].(map[string]any)["routing_strategy"])
	assert.Equal(t, true, cfg["litellm_settings"].(map[string]any)["cache"])
}

func TestRenderConfig_ExtraConfigMergesTopLevelKeys(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		Spec: litellmv1alpha1.LiteLLMProxySpec{
			RouterSettings: raw(`{"routing_strategy":"simple-shuffle"}`),
			ExtraConfig: raw(`{
				"guardrails":[{"guardrail_name":"g1"}],
				"mcp_servers":{"s":{"url":"http://x"}},
				"environment_variables":{"FOO":"bar"}
			}`),
		},
	}
	got, err := renderM(proxy, nil)
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	assert.Contains(t, cfg, "guardrails")
	assert.Contains(t, cfg, "mcp_servers")
	assert.Equal(t, "bar", cfg["environment_variables"].(map[string]any)["FOO"])
	// dedicated fields still render alongside extraConfig
	assert.Equal(t, "simple-shuffle", cfg["router_settings"].(map[string]any)["routing_strategy"])
}

func TestRenderConfig_ModelListWinsOverExtraConfig(t *testing.T) {
	m := model("a", "a", litellmv1alpha1.LiteLLMParams{Model: modelOpenAIM})
	proxy := &litellmv1alpha1.LiteLLMProxy{
		Spec: litellmv1alpha1.LiteLLMProxySpec{ExtraConfig: raw(`{"model_list":[{"model_name":"bogus"}]}`)},
	}
	got, err := renderM(proxy, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	list := parse(t, got.yaml)["model_list"].([]any)
	require.Len(t, list, 1)
	assert.Equal(t, "a", list[0].(map[string]any)[keyModelName])
}

func TestRenderConfig_OmitsUnsetGlobalSettingsAndOptionalParams(t *testing.T) {
	m := model("m", "m", litellmv1alpha1.LiteLLMParams{Model: modelOpenAIM})
	got, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	_, hasGeneral := cfg["general_settings"]
	_, hasRouter := cfg["router_settings"]
	_, hasLitellm := cfg["litellm_settings"]
	assert.False(t, hasGeneral)
	assert.False(t, hasRouter)
	assert.False(t, hasLitellm)

	params := cfg["model_list"].([]any)[0].(map[string]any)[keyLitellmParams].(map[string]any)
	for _, k := range []string{"api_base", "api_key", "api_version", "drop_params", "rpm", "tpm"} {
		_, ok := params[k]
		assert.Falsef(t, ok, "expected %q to be omitted when unset", k)
	}
	_, hasInfo := cfg["model_list"].([]any)[0].(map[string]any)["model_info"]
	assert.False(t, hasInfo)
}

func TestRenderConfig_RejectsCollidingAPIKeyEnvVars(t *testing.T) {
	ref := func(secret string) *litellmv1alpha1.SecretKeyRef {
		return &litellmv1alpha1.SecretKeyRef{Name: secret, Key: secretKeyKey}
	}
	// "minimax-m3" and "minimax.m3" both sanitize to LITELLM_MODELKEY_MINIMAX_M3.
	dash := model("minimax-m3", "a", litellmv1alpha1.LiteLLMParams{Model: "openai/a", APIKeyRef: ref("s1")})
	dot := model("minimax.m3", "b", litellmv1alpha1.LiteLLMParams{Model: "openai/b", APIKeyRef: ref("s2")})

	_, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{dash, dot})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same env var")
}

func TestRenderConfig_InvalidJSONErrors(t *testing.T) {
	m := model("m", "m", litellmv1alpha1.LiteLLMParams{Model: modelOpenAIM, Additional: raw(`{not json`)})
	_, err := renderM(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "m")
}

func TestRenderConfig_GuardrailsRenderedWithSecretRef(t *testing.T) {
	on := true
	g := litellmv1alpha1.LiteLLMGuardrail{
		ObjectMeta: metav1.ObjectMeta{Name: "aporia-pre"},
		Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
			GuardrailName: "aporia-pre-guard",
			Guardrail:     guardrailAporia,
			Mode:          "pre_call",
			DefaultOn:     &on,
			APIKeyRef:     &litellmv1alpha1.SecretKeyRef{Name: "gr", Key: "APORIA_KEY"},
			Info:          raw(`{"description":"pii"}`),
		},
	}
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, nil, []litellmv1alpha1.LiteLLMGuardrail{g}, nil)
	require.NoError(t, err)

	entry := parse(t, got.yaml)["guardrails"].([]any)[0].(map[string]any)
	assert.Equal(t, "aporia-pre-guard", entry["guardrail_name"])
	lp := entry[keyLitellmParams].(map[string]any)
	assert.Equal(t, "aporia", lp["guardrail"])
	assert.Equal(t, "pre_call", lp["mode"])
	assert.Equal(t, true, lp["default_on"])
	assert.Equal(t, "os.environ/LITELLM_GUARDRAILKEY_APORIA_PRE", lp["api_key"])
	assert.Equal(t, "pii", entry["guardrail_info"].(map[string]any)["description"])
	require.Len(t, got.envVars, 1)
	assert.Equal(t, "APORIA_KEY", got.envVars[0].ValueFrom.SecretKeyRef.Key)
}

func TestRenderConfig_MCPServersRenderedAsMap(t *testing.T) {
	s := litellmv1alpha1.LiteLLMMCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: "gh"},
		Spec: litellmv1alpha1.LiteLLMMCPServerSpec{
			Alias:        "github",
			URL:          "https://api.githubcopilot.com/mcp",
			Transport:    "http",
			AuthTokenRef: &litellmv1alpha1.SecretKeyRef{Name: "mcp", Key: "GH_PAT"},
			Params:       raw(`{"allowed_tools":["search"]}`),
		},
	}
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, nil, nil, []litellmv1alpha1.LiteLLMMCPServer{s})
	require.NoError(t, err)

	servers := parse(t, got.yaml)["mcp_servers"].(map[string]any)
	gh := servers["github"].(map[string]any)
	assert.Equal(t, "https://api.githubcopilot.com/mcp", gh["url"])
	assert.Equal(t, "http", gh["transport"])
	assert.Equal(t, "os.environ/LITELLM_MCPTOKEN_GH", gh["authentication_token"])
	assert.Contains(t, gh["allowed_tools"], "search")
	require.Len(t, got.envVars, 1)
}

func TestRenderConfig_CallbacksMergeIntoLitellmSettingsAndCallbackSettings(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		Spec: litellmv1alpha1.LiteLLMProxySpec{
			LitellmSettings: raw(`{"cache":true}`),
			Callbacks: &litellmv1alpha1.CallbackSpec{
				Success:  []string{"prometheus", "langfuse"},
				Failure:  []string{"sentry"},
				Settings: raw(`{"otel":{"exporter":"otlp"}}`),
			},
		},
	}
	got, err := renderConfig(proxy, nil, nil, nil)
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	ls := cfg["litellm_settings"].(map[string]any)
	assert.Equal(t, true, ls["cache"]) // preserved
	assert.Equal(t, []any{"prometheus", "langfuse"}, ls["success_callback"])
	assert.Equal(t, []any{"sentry"}, ls["failure_callback"])
	assert.Equal(t, "otlp", cfg["callback_settings"].(map[string]any)["otel"].(map[string]any)["exporter"])
}

func TestRenderConfig_NamedTopLevelBlocks(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		Spec: litellmv1alpha1.LiteLLMProxySpec{
			EnvironmentVariables: raw(`{"LANGFUSE_HOST":"https://lf"}`),
			CredentialList:       raw(`[{"credential_name":"c1"}]`),
		},
	}
	got, err := renderConfig(proxy, nil, nil, nil)
	require.NoError(t, err)

	cfg := parse(t, got.yaml)
	assert.Equal(t, "https://lf", cfg["environment_variables"].(map[string]any)["LANGFUSE_HOST"])
	// credential_list is a list, preserved as such
	assert.Equal(t, "c1", cfg["credential_list"].([]any)[0].(map[string]any)["credential_name"])
}

func TestRenderConfig_EnvCollisionAcrossKinds(t *testing.T) {
	m := model("dup", "m", litellmv1alpha1.LiteLLMParams{
		Model:     modelOpenAIM,
		APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
	})
	// A guardrail whose key env var would collide with... a different prefix, so no collision.
	// Instead, two models with names colliding is already covered; here assert distinct prefixes don't collide.
	g := litellmv1alpha1.LiteLLMGuardrail{
		ObjectMeta: metav1.ObjectMeta{Name: "dup"},
		Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
			GuardrailName: "dup", Guardrail: guardrailAporia,
			APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
		},
	}
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m}, []litellmv1alpha1.LiteLLMGuardrail{g}, nil)
	require.NoError(t, err)
	// model -> LITELLM_MODELKEY_DUP, guardrail -> LITELLM_GUARDRAILKEY_DUP (distinct)
	require.Len(t, got.envVars, 2)
}

func TestRenderConfig_APIModeSettingsOmitModelListAndEnableDBStore(t *testing.T) {
	m := model("glm", "glm-5.2", litellmv1alpha1.LiteLLMParams{
		Model:     "openai/glm-5.2",
		APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "s", Key: "k"},
	})
	g := litellmv1alpha1.LiteLLMGuardrail{
		ObjectMeta: metav1.ObjectMeta{Name: "gr"},
		Spec:       litellmv1alpha1.LiteLLMGuardrailSpec{GuardrailName: "g1", Guardrail: guardrailAporia},
	}
	got, err := renderConfig(&litellmv1alpha1.LiteLLMProxy{}, []litellmv1alpha1.LiteLLMModel{m}, []litellmv1alpha1.LiteLLMGuardrail{g}, nil)
	require.NoError(t, err)

	// File config carries the models; the api/settings config does not.
	full := parse(t, got.yaml)
	assert.Contains(t, full, "model_list")

	settings := parse(t, got.settingsYAML)
	_, hasModels := settings["model_list"]
	assert.False(t, hasModels, "api-mode config must omit model_list (models go via API)")
	assert.Equal(t, true, settings["general_settings"].(map[string]any)["store_model_in_db"])
	assert.Contains(t, settings, "guardrails") // guardrails stay in config even in api mode

	// The model's secret env var is still wired regardless of mode.
	require.Len(t, got.envVars, 1)
	assert.Equal(t, "LITELLM_MODELKEY_GLM", got.envVars[0].Name)
	// And the model entry is exposed for the API push.
	require.Len(t, got.models, 1)
	assert.Equal(t, "glm-5.2", got.models[0][keyModelName])
}

func ptr[T any](v T) *T { return &v }

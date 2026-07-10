package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	appName          = "litellm"
	configFileName   = "config.yaml"
	configMountPath  = litellmv1alpha1.ProxyConfigMountPath
	configVolumeName = litellmv1alpha1.ProxyConfigVolumeName
	proxyContainer   = appName
	proxyPort        = 4000
	httpPortName     = "http"

	conditionTypeReady = "Ready"

	keyModel         = "model"
	keyModelName     = "model_name"
	keyLitellmParams = "litellm_params"
)

// renderedConfig is the output of folding a proxy and its resources into a config.yaml.
// In file mode the controller applies `yaml`/`hash`; in api mode it applies the
// settings-only `settingsYAML`/`settingsHash` and pushes the entries via the API.
type renderedConfig struct {
	yaml         string
	hash         string
	settingsYAML string
	settingsHash string
	envVars      []corev1.EnvVar
	models       []map[string]any
	guardrails   []map[string]any
	mcpServers   map[string]any
}

// envAccumulator collects the secret-backed env vars wired into the proxy
// Deployment and guards against two resources deriving the same env var name.
type envAccumulator struct {
	vars  []corev1.EnvVar
	owner map[string]string
}

// secretParam sets params[key] from either a Secret ref (wiring an env var and
// referencing it via os.environ) or a literal, leaving it unset when neither is given.
func (a *envAccumulator) secretParam(params map[string]any, key, literal string, ref *litellmv1alpha1.SecretKeyRef, envName, ownerName string) error {
	switch {
	case ref != nil:
		if prev, ok := a.owner[envName]; ok {
			return fmt.Errorf("%q and %q derive the same env var %q", prev, ownerName, envName)
		}
		a.owner[envName] = ownerName
		params[key] = "os.environ/" + envName
		a.vars = append(a.vars, secretEnv(envName, ref))
	case literal != "":
		params[key] = literal
	}
	return nil
}

// renderConfig builds the proxy's config.yaml deterministically from the adopted
// models, guardrails and MCP servers plus the proxy settings. Collections are
// sorted by name so the output (and its hash) is stable regardless of List order.
// Secret-backed values are emitted as os.environ references and returned as env
// vars to wire into the Deployment, keeping secret values out of the ConfigMap.
func renderConfig(
	proxy *litellmv1alpha1.LiteLLMProxy,
	models []litellmv1alpha1.LiteLLMModel,
	guardrails []litellmv1alpha1.LiteLLMGuardrail,
	mcpServers []litellmv1alpha1.LiteLLMMCPServer,
) (renderedConfig, error) {
	env := &envAccumulator{owner: map[string]string{}}

	modelEntries, err := renderModels(models, env)
	if err != nil {
		return renderedConfig{}, err
	}
	guardrailEntries, err := renderGuardrails(guardrails, env)
	if err != nil {
		return renderedConfig{}, err
	}
	mcpEntries, err := renderMCPServers(mcpServers, env)
	if err != nil {
		return renderedConfig{}, err
	}

	// Full config (file mode): settings plus the rendered collections.
	full, err := buildBaseConfig(proxy)
	if err != nil {
		return renderedConfig{}, err
	}
	full["model_list"] = modelEntries
	if guardrailEntries != nil {
		full["guardrails"] = guardrailEntries
	}
	if mcpEntries != nil {
		full["mcp_servers"] = mcpEntries
	}
	fullYAML, err := marshalYAML(full)
	if err != nil {
		return renderedConfig{}, err
	}

	// Api-mode config: everything except model_list (models are pushed live via
	// the admin API), with the DB model store turned on.
	settings, err := buildBaseConfig(proxy)
	if err != nil {
		return renderedConfig{}, err
	}
	if guardrailEntries != nil {
		settings["guardrails"] = guardrailEntries
	}
	if mcpEntries != nil {
		settings["mcp_servers"] = mcpEntries
	}
	gs, _ := settings["general_settings"].(map[string]any)
	if gs == nil {
		gs = map[string]any{}
	}
	gs["store_model_in_db"] = true
	settings["general_settings"] = gs
	settingsYAML, err := marshalYAML(settings)
	if err != nil {
		return renderedConfig{}, err
	}

	return renderedConfig{
		yaml:         fullYAML,
		hash:         hashString(fullYAML),
		settingsYAML: settingsYAML,
		settingsHash: hashString(settingsYAML),
		envVars:      env.vars,
		models:       modelEntries,
		guardrails:   guardrailEntries,
		mcpServers:   mcpEntries,
	}, nil
}

// buildBaseConfig assembles everything except the rendered collections: the
// extraConfig catch-all, the named top-level blocks, the three settings blocks,
// and callbacks.
func buildBaseConfig(proxy *litellmv1alpha1.LiteLLMProxy) (map[string]any, error) {
	config, err := decodeRaw(proxy.Spec.ExtraConfig)
	if err != nil {
		return nil, fmt.Errorf("extraConfig: %w", err)
	}
	if config == nil {
		config = map[string]any{}
	}
	for key, raw := range topLevelBlocks(proxy) {
		if err := mergeValue(config, key, raw); err != nil {
			return nil, err
		}
	}
	if err := applyCallbacks(config, proxy.Spec.Callbacks); err != nil {
		return nil, err
	}
	return config, nil
}

func marshalYAML(config map[string]any) (string, error) {
	out, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(out), nil
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func renderModels(models []litellmv1alpha1.LiteLLMModel, env *envAccumulator) ([]map[string]any, error) {
	sorted := append([]litellmv1alpha1.LiteLLMModel(nil), models...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	list := make([]map[string]any, 0, len(sorted))
	for i := range sorted {
		m := &sorted[i]
		params, err := decodeRaw(m.Spec.Params.Additional)
		if err != nil {
			return nil, fmt.Errorf("model %q params: %w", m.Name, err)
		}
		if params == nil {
			params = map[string]any{}
		}
		p := m.Spec.Params
		params[keyModel] = p.Model
		if p.APIVersion != "" {
			params["api_version"] = p.APIVersion
		}
		if p.DropParams != nil {
			params["drop_params"] = *p.DropParams
		}
		if p.RPM != nil {
			params["rpm"] = *p.RPM
		}
		if p.TPM != nil {
			params["tpm"] = *p.TPM
		}
		if err := env.secretParam(params, "api_base", p.APIBase, p.APIBaseRef, m.APIBaseEnvVarName(), m.Name); err != nil {
			return nil, err
		}
		if err := env.secretParam(params, "api_key", p.APIKey, p.APIKeyRef, m.APIKeyEnvVarName(), m.Name); err != nil {
			return nil, err
		}

		entry := map[string]any{keyModelName: m.Spec.ModelName, keyLitellmParams: params}
		info, err := renderModelInfo(m.Spec.Info)
		if err != nil {
			return nil, fmt.Errorf("model %q info: %w", m.Name, err)
		}
		if info != nil {
			entry["model_info"] = info
		}
		list = append(list, entry)
	}
	return list, nil
}

func renderGuardrails(guardrails []litellmv1alpha1.LiteLLMGuardrail, env *envAccumulator) ([]map[string]any, error) {
	if len(guardrails) == 0 {
		return nil, nil
	}
	sorted := append([]litellmv1alpha1.LiteLLMGuardrail(nil), guardrails...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	list := make([]map[string]any, 0, len(sorted))
	for i := range sorted {
		g := &sorted[i]
		params, err := decodeRaw(g.Spec.Params)
		if err != nil {
			return nil, fmt.Errorf("guardrail %q params: %w", g.Name, err)
		}
		if params == nil {
			params = map[string]any{}
		}
		params["guardrail"] = g.Spec.Guardrail
		if g.Spec.Mode != "" {
			params["mode"] = g.Spec.Mode
		}
		if g.Spec.DefaultOn != nil {
			params["default_on"] = *g.Spec.DefaultOn
		}
		if err := env.secretParam(params, "api_base", g.Spec.APIBase, g.Spec.APIBaseRef, g.APIBaseEnvVarName(), g.Name); err != nil {
			return nil, err
		}
		if err := env.secretParam(params, "api_key", g.Spec.APIKey, g.Spec.APIKeyRef, g.APIKeyEnvVarName(), g.Name); err != nil {
			return nil, err
		}

		entry := map[string]any{"guardrail_name": g.Spec.GuardrailName, keyLitellmParams: params}
		info, err := decodeRaw(g.Spec.Info)
		if err != nil {
			return nil, fmt.Errorf("guardrail %q info: %w", g.Name, err)
		}
		if info != nil {
			entry["guardrail_info"] = info
		}
		list = append(list, entry)
	}
	return list, nil
}

func renderMCPServers(servers []litellmv1alpha1.LiteLLMMCPServer, env *envAccumulator) (map[string]any, error) {
	if len(servers) == 0 {
		return nil, nil
	}
	sorted := append([]litellmv1alpha1.LiteLLMMCPServer(nil), servers...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	out := map[string]any{}
	for i := range sorted {
		s := &sorted[i]
		entry, err := decodeRaw(s.Spec.Params)
		if err != nil {
			return nil, fmt.Errorf("mcp server %q params: %w", s.Name, err)
		}
		if entry == nil {
			entry = map[string]any{}
		}
		if url := s.ResolvedServerURL(); url != "" {
			entry["url"] = url
		}
		transport := s.Spec.Transport
		if transport == "" && s.Spec.Workload != nil {
			transport = "http"
		}
		if transport != "" {
			entry["transport"] = transport
		}
		if s.Spec.AuthType != "" {
			entry["auth_type"] = s.Spec.AuthType
		}
		if err := env.secretParam(entry, "authentication_token", "", s.Spec.AuthTokenRef, s.AuthTokenEnvVarName(), s.Name); err != nil {
			return nil, err
		}
		alias := s.ServerAlias()
		if _, dup := out[alias]; dup {
			return nil, fmt.Errorf("mcp server %q reuses alias %q", s.Name, alias)
		}
		out[alias] = entry
	}
	return out, nil
}

// applyCallbacks overlays the callback lists onto litellm_settings and sets the
// top-level callback_settings block.
func applyCallbacks(config map[string]any, cb *litellmv1alpha1.CallbackSpec) error {
	if cb == nil {
		return nil
	}
	if len(cb.Success) > 0 || len(cb.Failure) > 0 || len(cb.Callbacks) > 0 {
		ls, _ := config["litellm_settings"].(map[string]any)
		if ls == nil {
			ls = map[string]any{}
		}
		if len(cb.Success) > 0 {
			ls["success_callback"] = cb.Success
		}
		if len(cb.Failure) > 0 {
			ls["failure_callback"] = cb.Failure
		}
		if len(cb.Callbacks) > 0 {
			ls["callbacks"] = cb.Callbacks
		}
		config["litellm_settings"] = ls
	}
	return mergeValue(config, "callback_settings", cb.Settings)
}

// topLevelBlocks maps each named passthrough field to its config.yaml key.
func topLevelBlocks(proxy *litellmv1alpha1.LiteLLMProxy) map[string]*runtime.RawExtension {
	s := proxy.Spec
	return map[string]*runtime.RawExtension{
		"general_settings":      s.GeneralSettings,
		"router_settings":       s.RouterSettings,
		"litellm_settings":      s.LitellmSettings,
		"environment_variables": s.EnvironmentVariables,
		"credential_list":       s.CredentialList,
		"default_vertex_config": s.DefaultVertexConfig,
		"files_settings":        s.FilesSettings,
		"assistant_settings":    s.AssistantSettings,
		"finetune_settings":     s.FinetuneSettings,
		"prompts":               s.Prompts,
		"vector_store_registry": s.VectorStoreRegistry,
	}
}

func mergeValue(config map[string]any, key string, raw *runtime.RawExtension) error {
	v, err := decodeRawValue(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if v != nil {
		config[key] = v
	}
	return nil
}

func secretEnv(name string, ref *litellmv1alpha1.SecretKeyRef) corev1.EnvVar {
	return corev1.EnvVar{
		Name: name,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: ref.Name},
				Key:                  ref.Key,
			},
		},
	}
}

// renderModelInfo flattens the typed ModelInfo (plus its Extra escape hatch)
// into the snake_case map litellm expects under model_info.
func renderModelInfo(info *litellmv1alpha1.ModelInfo) (map[string]any, error) {
	if info == nil {
		return nil, nil
	}
	out, err := decodeRaw(info.Extra)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	if info.MaxTokens != nil {
		out["max_tokens"] = *info.MaxTokens
	}
	if info.MaxInputTokens != nil {
		out["max_input_tokens"] = *info.MaxInputTokens
	}
	if info.MaxOutputTokens != nil {
		out["max_output_tokens"] = *info.MaxOutputTokens
	}
	if info.Mode != "" {
		out["mode"] = info.Mode
	}
	if info.SupportsFunctionCalling != nil {
		out["supports_function_calling"] = *info.SupportsFunctionCalling
	}
	if info.SupportsPromptCaching != nil {
		out["supports_prompt_caching"] = *info.SupportsPromptCaching
	}
	if info.SupportsVision != nil {
		out["supports_vision"] = *info.SupportsVision
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func decodeRaw(raw *runtime.RawExtension) (map[string]any, error) {
	v, err := decodeRawValue(raw)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected an object, got %T", v)
	}
	return m, nil
}

func decodeRawValue(raw *runtime.RawExtension) (any, error) {
	if raw == nil || len(raw.Raw) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw.Raw))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

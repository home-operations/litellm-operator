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
	appName         = "litellm"
	configFileName  = "config.yaml"
	configMountPath = "/etc/litellm"
	proxyContainer  = appName
	proxyPort       = 4000
)

// renderedConfig is the output of folding a proxy and its models into a config.yaml.
type renderedConfig struct {
	yaml    string
	hash    string
	envVars []corev1.EnvVar
}

// renderConfig builds the proxy's config.yaml deterministically from the given
// models and proxy settings. Models are sorted by name so the output (and its
// hash) is stable regardless of List ordering. Secret-backed API keys are
// emitted as os.environ references and returned as env vars to wire into the
// Deployment, keeping secret values out of the ConfigMap.
func renderConfig(proxy *litellmv1alpha1.LiteLLMProxy, models []litellmv1alpha1.LiteLLMModel) (renderedConfig, error) {
	sorted := make([]litellmv1alpha1.LiteLLMModel, len(models))
	copy(sorted, models)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	modelList := make([]map[string]any, 0, len(sorted))
	envVars := make([]corev1.EnvVar, 0, len(sorted))
	envOwner := make(map[string]string, len(sorted))

	for i := range sorted {
		m := &sorted[i]
		params, err := decodeRaw(m.Spec.Params.Additional)
		if err != nil {
			return renderedConfig{}, fmt.Errorf("model %q additional params: %w", m.Name, err)
		}
		if params == nil {
			params = map[string]any{}
		}

		p := m.Spec.Params
		params["model"] = p.Model
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

		switch {
		case p.APIBaseRef != nil:
			envName := m.APIBaseEnvVarName()
			if err := claimEnv(envOwner, envName, m.Name); err != nil {
				return renderedConfig{}, err
			}
			params["api_base"] = "os.environ/" + envName
			envVars = append(envVars, secretEnv(envName, p.APIBaseRef))
		case p.APIBase != "":
			params["api_base"] = p.APIBase
		}

		switch {
		case p.APIKeyRef != nil:
			envName := m.APIKeyEnvVarName()
			if err := claimEnv(envOwner, envName, m.Name); err != nil {
				return renderedConfig{}, err
			}
			params["api_key"] = "os.environ/" + envName
			envVars = append(envVars, secretEnv(envName, p.APIKeyRef))
		case p.APIKey != "":
			params["api_key"] = p.APIKey
		}

		entry := map[string]any{
			"model_name":     m.Spec.ModelName,
			"litellm_params": params,
		}
		info, err := renderModelInfo(m.Spec.Info)
		if err != nil {
			return renderedConfig{}, fmt.Errorf("model %q info: %w", m.Name, err)
		}
		if info != nil {
			entry["model_info"] = info
		}
		modelList = append(modelList, entry)
	}

	config := map[string]any{"model_list": modelList}
	if err := mergeSettings(config, "general_settings", proxy.Spec.GeneralSettings); err != nil {
		return renderedConfig{}, err
	}
	if err := mergeSettings(config, "router_settings", proxy.Spec.RouterSettings); err != nil {
		return renderedConfig{}, err
	}
	if err := mergeSettings(config, "litellm_settings", proxy.Spec.LitellmSettings); err != nil {
		return renderedConfig{}, err
	}

	out, err := yaml.Marshal(config)
	if err != nil {
		return renderedConfig{}, fmt.Errorf("marshal config: %w", err)
	}
	sum := sha256.Sum256(out)
	return renderedConfig{
		yaml:    string(out),
		hash:    hex.EncodeToString(sum[:]),
		envVars: envVars,
	}, nil
}

func mergeSettings(config map[string]any, key string, raw *runtime.RawExtension) error {
	settings, err := decodeRaw(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if settings != nil {
		config[key] = settings
	}
	return nil
}

// claimEnv records that model owns the env var name, erroring if another model
// already claimed it. This backstops the admission webhook: two models whose
// names sanitize to the same env var would otherwise silently clobber each other.
func claimEnv(owner map[string]string, name, model string) error {
	if prev, ok := owner[name]; ok {
		return fmt.Errorf("models %q and %q derive the same env var %q", prev, model, name)
	}
	owner[name] = model
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
	if raw == nil || len(raw.Raw) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw.Raw))
	dec.UseNumber()
	out := map[string]any{}
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

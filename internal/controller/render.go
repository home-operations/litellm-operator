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
	configFileName  = "config.yaml"
	configMountPath = "/etc/litellm"
	proxyContainer  = "litellm"
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
		if p.APIBase != "" {
			params["api_base"] = p.APIBase
		}
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
		case p.APIKeyRef != nil:
			envName := m.APIKeyEnvVarName()
			params["api_key"] = "os.environ/" + envName
			envVars = append(envVars, corev1.EnvVar{
				Name: envName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: p.APIKeyRef.Name},
						Key:                  p.APIKeyRef.Key,
					},
				},
			})
		case p.APIKey != "":
			params["api_key"] = p.APIKey
		}

		entry := map[string]any{
			"model_name":     m.Spec.ModelName,
			"litellm_params": params,
		}
		info, err := decodeRaw(m.Spec.Info)
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

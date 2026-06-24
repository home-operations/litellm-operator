package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const nameMinimaxM3 = "minimax-m3"

func TestAPIKeyEnvVarName(t *testing.T) {
	cases := map[string]string{
		"glm":          "LITELLM_MODELKEY_GLM",
		nameMinimaxM3:  "LITELLM_MODELKEY_MINIMAX_M3",
		"qwen3.6-27b":  "LITELLM_MODELKEY_QWEN3_6_27B",
		"neuralwatt-x": "LITELLM_MODELKEY_NEURALWATT_X",
	}
	for name, want := range cases {
		m := &LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: name}}
		assert.Equalf(t, want, m.APIKeyEnvVarName(), "input %q", name)
	}
}

func TestAPIKeyEnvVarName_DistinctNamesDistinctVars(t *testing.T) {
	a := &LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: nameMinimaxM3}}
	b := &LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: "minimax-m2"}}
	assert.NotEqual(t, a.APIKeyEnvVarName(), b.APIKeyEnvVarName())
}

func TestAPIKeyEnvVarName_PunctuationCollapsesToSameVar(t *testing.T) {
	// '.' and '-' both sanitize to '_', so these collide — the proxy webhook
	// rejects such a pair to prevent one model's key silently overwriting another's.
	dash := &LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: nameMinimaxM3}}
	dot := &LiteLLMModel{ObjectMeta: metav1.ObjectMeta{Name: "minimax.m3"}}
	assert.Equal(t, dash.APIKeyEnvVarName(), dot.APIKeyEnvVarName())
}

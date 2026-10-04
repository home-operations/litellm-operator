package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const testProxyNamespace = "ai"

func TestProxyResourceSelection(t *testing.T) {
	selectedLabels := map[string]string{"selection": testProxyName}
	selector := &metav1.LabelSelector{MatchLabels: selectedLabels}
	tests := []struct {
		name           string
		namespace      string
		proxyRef       string
		proxyNamespace string
		labels         map[string]string
		selector       *metav1.LabelSelector
		want           bool
	}{
		{name: "local default", namespace: testProxyNamespace, want: true},
		{name: "foreign default", namespace: testTeamName},
		{name: "local reference", namespace: testProxyNamespace, proxyRef: testProxyName, want: true},
		{name: "foreign reference without namespace", namespace: testTeamName, proxyRef: testProxyName},
		{name: "cross namespace reference", namespace: testTeamName, proxyRef: testProxyName, proxyNamespace: testProxyNamespace, want: true},
		{name: "different proxy", namespace: testProxyNamespace, proxyRef: "another-proxy"},
		{name: "different namespace", namespace: testProxyNamespace, proxyRef: testProxyName, proxyNamespace: testTeamName},
		{name: "matching selector", namespace: testProxyNamespace, labels: selectedLabels, selector: selector, want: true},
		{name: "foreign matching selector", namespace: testTeamName, labels: selectedLabels, selector: selector},
		{name: "nonmatching selector", namespace: testProxyNamespace, selector: selector},
		{name: "reference overrides selector", namespace: testTeamName, proxyRef: testProxyName, proxyNamespace: testProxyNamespace, selector: selector, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &litellmv1alpha1.LiteLLMProxy{
				ObjectMeta: metav1.ObjectMeta{Name: testProxyName, Namespace: testProxyNamespace},
				Spec:       litellmv1alpha1.LiteLLMProxySpec{ModelSelector: tt.selector},
			}
			meta := metav1.ObjectMeta{Name: "resource", Namespace: tt.namespace, Labels: tt.labels}
			model := &litellmv1alpha1.LiteLLMModel{ObjectMeta: meta, Spec: litellmv1alpha1.LiteLLMModelSpec{ProxyRef: tt.proxyRef, ProxyNamespace: tt.proxyNamespace}}
			guardrail := &litellmv1alpha1.LiteLLMGuardrail{ObjectMeta: meta, Spec: litellmv1alpha1.LiteLLMGuardrailSpec{ProxyRef: tt.proxyRef, ProxyNamespace: tt.proxyNamespace}}
			server := &litellmv1alpha1.LiteLLMMCPServer{ObjectMeta: meta, Spec: litellmv1alpha1.LiteLLMMCPServerSpec{ProxyRef: tt.proxyRef, ProxyNamespace: tt.proxyNamespace}}
			scheme := runtime.NewScheme()
			require.NoError(t, litellmv1alpha1.AddToScheme(scheme))
			r := &LiteLLMProxyReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(proxy, model, guardrail, server).Build()}

			models, err := r.matchingModels(t.Context(), proxy)
			require.NoError(t, err)
			assert.Equal(t, tt.want, len(models) == 1)
			guardrails, err := r.matchingGuardrails(t.Context(), proxy)
			require.NoError(t, err)
			assert.Equal(t, tt.want, len(guardrails) == 1)
			servers, err := r.matchingMCPServers(t.Context(), proxy)
			require.NoError(t, err)
			assert.Equal(t, tt.want, len(servers) == 1)

			for _, obj := range []client.Object{model, guardrail, server} {
				requests := r.proxiesForObject(t.Context(), obj)
				if tt.want {
					assert.Equal(t, []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(proxy)}}, requests)
				} else {
					assert.Empty(t, requests)
				}
			}
		})
	}
}

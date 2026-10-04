package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var _ = Describe("Cross-namespace proxy references", func() {
	It("reconciles MCP creation, updates, rebinding and deletion across namespaces", func() {
		const (
			appsNamespace  = "reference-apps"
			proxyNamespace = "reference-ai"
			proxyName      = "shared"
			serverName     = "remote-mcp"
		)
		proxyConfigKey := types.NamespacedName{Name: proxyName + "-config", Namespace: proxyNamespace}
		appsConfigKey := types.NamespacedName{Name: proxyName + "-config", Namespace: appsNamespace}
		for _, namespace := range []string{appsNamespace, proxyNamespace} {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
			Expect(k8sClient.Create(ctx, &litellmv1alpha1.LiteLLMProxy{
				ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: namespace},
			})).To(Succeed())
			Eventually(func(g Gomega) {
				var cm corev1.ConfigMap
				key := types.NamespacedName{Name: proxyName + "-config", Namespace: namespace}
				g.Expect(k8sClient.Get(ctx, key, &cm)).To(Succeed())
			}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		}
		server := &litellmv1alpha1.LiteLLMMCPServer{
			ObjectMeta: metav1.ObjectMeta{Name: serverName, Namespace: appsNamespace},
			Spec: litellmv1alpha1.LiteLLMMCPServerSpec{
				ProxyRef: proxyName, ProxyNamespace: proxyNamespace,
				Workload: &litellmv1alpha1.MCPWorkloadSpec{Image: "mcp/grafana:latest", Port: 8000},
			},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, proxyConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("http://remote-mcp.reference-apps.svc.cluster.local:8000/mcp"))
			g.Expect(k8sClient.Get(ctx, appsConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).NotTo(ContainSubstring(serverName))
			var deploy appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &deploy)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&deploy, server)).To(BeTrue())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), server)).To(Succeed())
		server.Spec.Workload.Port = 8001
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, proxyConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring(":8001/mcp"))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), server)).To(Succeed())
		server.Spec.ProxyNamespace = appsNamespace
		Expect(k8sClient.Update(ctx, server)).To(Succeed())
		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, proxyConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).NotTo(ContainSubstring(serverName))
			g.Expect(k8sClient.Get(ctx, appsConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring(serverName))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, appsConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).NotTo(ContainSubstring(serverName))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("requires a proxy name when setting a proxy namespace", func() {
		meta := metav1.ObjectMeta{Name: "invalid-proxy-namespace", Namespace: metav1.NamespaceDefault}
		resources := []client.Object{
			&litellmv1alpha1.LiteLLMModel{ObjectMeta: meta, Spec: litellmv1alpha1.LiteLLMModelSpec{
				ProxyNamespace: "ai", ModelName: "model", Params: litellmv1alpha1.LiteLLMParams{Model: "openai/model"},
			}},
			&litellmv1alpha1.LiteLLMGuardrail{ObjectMeta: meta, Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				ProxyNamespace: "ai", GuardrailName: "guardrail", Guardrail: "presidio",
			}},
			&litellmv1alpha1.LiteLLMMCPServer{ObjectMeta: meta, Spec: litellmv1alpha1.LiteLLMMCPServerSpec{
				ProxyNamespace: "ai", URL: "https://example.com/mcp",
			}},
		}
		for _, resource := range resources {
			Expect(k8sClient.Create(ctx, resource)).To(MatchError(ContainSubstring("proxyNamespace requires proxyRef")))
		}
	})
})

package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var _ = Describe("Cross-namespace proxy references", func() {
	It("reconciles MCP creation, updates, rebinding and deletion across namespaces", func() {
		const (
			appsNamespace         = "reference-apps"
			proxyNamespace        = "reference-ai"
			proxyName             = "shared"
			serverName            = "remote-mcp"
			tokenKey              = "token"
			credentialsAnnotation = "litellm.home-operations.com/credentials-hash"
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
				AuthTokenRef: &litellmv1alpha1.SecretKeyRef{Name: "mcp-token", Key: tokenKey},
				Workload:     &litellmv1alpha1.MCPWorkloadSpec{Image: "mcp/grafana:latest", Port: 8000},
			},
		}
		source := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "mcp-token", Namespace: appsNamespace},
			Data:       map[string][]byte{tokenKey: []byte("initial-token"), "unused": []byte("do-not-copy")},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Eventually(func(g Gomega) {
			var proxy litellmv1alpha1.LiteLLMProxy
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName, Namespace: proxyNamespace}, &proxy)).To(Succeed())
			condition := meta.FindStatusCondition(proxy.Status.Conditions, "Ready")
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal("SecretResolutionFailed"))
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		Expect(k8sClient.Create(ctx, source)).To(Succeed())
		credentialsKey := types.NamespacedName{Name: proxyName + "-credentials", Namespace: proxyNamespace}
		var credentialsHash string
		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, proxyConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("http://remote-mcp.reference-apps.svc.cluster.local:8000/mcp"))
			g.Expect(cm.Data["config.yaml"]).NotTo(ContainSubstring("initial-token"))
			g.Expect(k8sClient.Get(ctx, appsConfigKey, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).NotTo(ContainSubstring(serverName))
			var deploy appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &deploy)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&deploy, server)).To(BeTrue())
			var credentials corev1.Secret
			g.Expect(k8sClient.Get(ctx, credentialsKey, &credentials)).To(Succeed())
			g.Expect(credentials.Data).To(Equal(map[string][]byte{server.AuthTokenEnvVarName(): []byte("initial-token")}))
			var proxy litellmv1alpha1.LiteLLMProxy
			proxyKey := types.NamespacedName{Name: proxyName, Namespace: proxyNamespace}
			g.Expect(k8sClient.Get(ctx, proxyKey, &proxy)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&credentials, &proxy)).To(BeTrue())
			g.Expect(k8sClient.Get(ctx, proxyKey, &deploy)).To(Succeed())
			g.Expect(deploy.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name: server.AuthTokenEnvVarName(),
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: credentialsKey.Name},
					Key:                  server.AuthTokenEnvVarName(),
				}},
			}))
			credentialsHash = deploy.Spec.Template.Annotations[credentialsAnnotation]
			g.Expect(credentialsHash).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(source), source)).To(Succeed())
		source.Data[tokenKey] = []byte("rotated-token")
		Expect(k8sClient.Update(ctx, source)).To(Succeed())
		Eventually(func(g Gomega) {
			var credentials corev1.Secret
			g.Expect(k8sClient.Get(ctx, credentialsKey, &credentials)).To(Succeed())
			g.Expect(credentials.Data[server.AuthTokenEnvVarName()]).To(Equal([]byte("rotated-token")))
			var deploy appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName, Namespace: proxyNamespace}, &deploy)).To(Succeed())
			g.Expect(deploy.Spec.Template.Annotations[credentialsAnnotation]).NotTo(Equal(credentialsHash))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name: credentialsKey.Name, Namespace: credentialsKey.Namespace,
		}})).To(Succeed())
		Eventually(func(g Gomega) {
			var credentials corev1.Secret
			g.Expect(k8sClient.Get(ctx, credentialsKey, &credentials)).To(Succeed())
			g.Expect(credentials.Data[server.AuthTokenEnvVarName()]).To(Equal([]byte("rotated-token")))
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
			var credentials corev1.Secret
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, credentialsKey, &credentials))).To(BeTrue())
			var deploy appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName, Namespace: appsNamespace}, &deploy)).To(Succeed())
			g.Expect(deploy.Spec.Template.Annotations).NotTo(HaveKey(credentialsAnnotation))
			g.Expect(deploy.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name: server.AuthTokenEnvVarName(),
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: source.Name}, Key: tokenKey,
				}},
			}))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		var unchangedSource corev1.Secret
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(source), &unchangedSource)).To(Succeed())
		Expect(unchangedSource.Data).To(Equal(source.Data))

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

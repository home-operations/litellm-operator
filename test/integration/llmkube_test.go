package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var _ = Describe("LLMKube auto-registration", func() {
	const (
		ns           = "default"
		managedLabel = "litellm.home-operations.com/managed-by"
	)

	// makeReady creates an InferenceService and a backing Model, then drives both
	// to Ready via the status subresource. Returns the InferenceService name.
	makeReady := func(name, modelRef string, ctxLen uint64) {
		model := &inferencev1alpha1.Model{
			ObjectMeta: metav1.ObjectMeta{Name: modelRef, Namespace: ns},
			Spec:       inferencev1alpha1.ModelSpec{Source: "https://example.com/m.gguf"},
		}
		Expect(k8sClient.Create(ctx, model)).To(Succeed())
		model.Status.Phase = "Ready"
		model.Status.GGUF = &inferencev1alpha1.GGUFMetadata{ContextLength: ctxLen}
		Expect(k8sClient.Status().Update(ctx, model)).To(Succeed())

		isvc := &inferencev1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec:       inferencev1alpha1.InferenceServiceSpec{ModelRef: modelRef},
		}
		Expect(k8sClient.Create(ctx, isvc)).To(Succeed())
		isvc.Status.Phase = "Ready"
		isvc.Status.Endpoint = "http://" + name + "." + ns + ".svc.cluster.local:8080/v1/chat/completions"
		Expect(k8sClient.Status().Update(ctx, isvc)).To(Succeed())
	}

	It("projects a Ready InferenceService into a LiteLLMModel a proxy adopts", func() {
		makeReady("llama3", "llama3-8b", 131072)

		var model litellmv1alpha1.LiteLLMModel
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "llama3", Namespace: ns}, &model)).To(Succeed())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(model.Labels[managedLabel]).To(Equal("llmkube"))
		Expect(model.Spec.ModelName).To(Equal("llama3"))
		Expect(model.Spec.Params.Model).To(Equal("openai/llama3-8b"))
		Expect(model.Spec.Params.APIBase).To(Equal("http://llama3.default.svc.cluster.local:8080/v1"))
		Expect(model.Spec.Params.APIKey).NotTo(BeEmpty())
		Expect(model.Spec.Info).NotTo(BeNil())
		Expect(model.Spec.Info.MaxInputTokens).NotTo(BeNil())
		Expect(*model.Spec.Info.MaxInputTokens).To(Equal(int64(131072)))

		By("being adopted by a proxy that selects managed-by=llmkube")
		proxy := &litellmv1alpha1.LiteLLMProxy{
			ObjectMeta: metav1.ObjectMeta{Name: "llmkube-proxy", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMProxySpec{
				ModelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{managedLabel: "llmkube"}},
				Service:       litellmv1alpha1.ProxyServiceSpec{Port: 4000},
			},
		}
		Expect(k8sClient.Create(ctx, proxy)).To(Succeed())

		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "llmkube-proxy-config", Namespace: ns}, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("openai/llama3-8b"))
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("llama3.default.svc.cluster.local:8080/v1"))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("deletes the generated model when the InferenceService reaches a terminal phase", func() {
		makeReady("doomed", "doomed-model", 4096)

		key := types.NamespacedName{Name: "doomed", Namespace: ns}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &litellmv1alpha1.LiteLLMModel{})).To(Succeed())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		var isvc inferencev1alpha1.InferenceService
		Expect(k8sClient.Get(ctx, key, &isvc)).To(Succeed())
		isvc.Status.Phase = "Failed"
		isvc.Status.Endpoint = ""
		Expect(k8sClient.Status().Update(ctx, &isvc)).To(Succeed())

		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, key, &litellmv1alpha1.LiteLLMModel{})
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(err).To(HaveOccurred())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("keeps the generated model through a transient (non-terminal) phase dip", func() {
		makeReady("rolling", "rolling-model", 4096)

		key := types.NamespacedName{Name: "rolling", Namespace: ns}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &litellmv1alpha1.LiteLLMModel{})).To(Succeed())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		var isvc inferencev1alpha1.InferenceService
		Expect(k8sClient.Get(ctx, key, &isvc)).To(Succeed())
		isvc.Status.Phase = "Progressing"
		Expect(k8sClient.Status().Update(ctx, &isvc)).To(Succeed())

		// A routine rollout must not delete the model out from under the proxy.
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &litellmv1alpha1.LiteLLMModel{})).To(Succeed())
		}, 3*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("never overwrites a pre-existing model the operator does not manage", func() {
		// A hand-written model whose name collides with an incoming InferenceService.
		existing := &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: "collide", Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				ModelName: "hand-written",
				Params:    litellmv1alpha1.LiteLLMParams{Model: "openai/keep-me"},
			},
		}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		makeReady("collide", "collide-model", 8192)

		// Give the reconciler time to (not) act, then confirm the model is untouched.
		Consistently(func(g Gomega) {
			var got litellmv1alpha1.LiteLLMModel
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "collide", Namespace: ns}, &got)).To(Succeed())
			g.Expect(got.Spec.Params.Model).To(Equal("openai/keep-me"))
			g.Expect(got.Labels).NotTo(HaveKey(managedLabel))
		}, 3*time.Second, 250*time.Millisecond).Should(Succeed())
	})
})

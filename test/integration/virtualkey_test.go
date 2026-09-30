package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var _ = Describe("LiteLLMVirtualKey schema", func() {
	It("stores MCP toolset assignments and allows clearing them", func() {
		key := &litellmv1alpha1.LiteLLMVirtualKey{
			ObjectMeta: metav1.ObjectMeta{Name: "toolset-key", Namespace: metav1.NamespaceDefault},
			Spec: litellmv1alpha1.LiteLLMVirtualKeySpec{
				ProxyRef: "main", SecretName: "toolset-key", MCPToolsets: []string{"toolset-a", "toolset-b"},
			},
		}
		Expect(k8sClient.Create(ctx, key)).To(Succeed())
		var got litellmv1alpha1.LiteLLMVirtualKey
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(key), &got)).To(Succeed())
		Expect(got.Spec.MCPToolsets).To(Equal(key.Spec.MCPToolsets))
		got.Spec.MCPToolsets = []string{}
		Expect(k8sClient.Update(ctx, &got)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(key), &got)).To(Succeed())
		Expect(got.Spec.MCPToolsets).To(BeEmpty())
	})
})

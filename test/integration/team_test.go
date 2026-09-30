package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var _ = Describe("LiteLLMTeam schema", func() {
	It("stores MCP server assignments and allows clearing them", func() {
		team := &litellmv1alpha1.LiteLLMTeam{
			ObjectMeta: metav1.ObjectMeta{Name: "mcp-team", Namespace: metav1.NamespaceDefault},
			Spec: litellmv1alpha1.LiteLLMTeamSpec{
				ProxyRef: "main", MCPServers: []string{"ha-mcp", "context7"},
			},
		}
		Expect(k8sClient.Create(ctx, team)).To(Succeed())
		var got litellmv1alpha1.LiteLLMTeam
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(team), &got)).To(Succeed())
		Expect(got.Spec.MCPServers).To(Equal(team.Spec.MCPServers))
		got.Spec.MCPServers = []string{}
		Expect(k8sClient.Update(ctx, &got)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(team), &got)).To(Succeed())
		Expect(got.Spec.MCPServers).To(BeEmpty())
	})
})

package e2e

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("cross-namespace credentials", func() {
	It("makes an MCP token available through the rendered proxy environment", func() {
		const (
			appsNamespace = "mcp-credentials"
			proxyName     = "credentials"
			podName       = "credential-consumer"
		)
		DeferCleanup(func() {
			_, _ = kubectl("delete", "pod", podName, "-n", testNS, "--ignore-not-found")
			_, _ = kubectl("delete", "litellmproxy", proxyName, "-n", testNS, "--ignore-not-found")
			_, _ = kubectl("delete", "namespace", appsNamespace, "--ignore-not-found", "--wait=false")
		})
		_, err := kubectlApply(fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: Secret
metadata:
  name: mcp-token
  namespace: %s
stringData:
  token: test-mcp-token
---
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMProxy
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 0
  modelSelector:
    matchLabels: {proxy: credentials}
---
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMMCPServer
metadata:
  name: remote
  namespace: %s
spec:
  proxyRef: %s
  proxyNamespace: %s
  url: https://example.com/mcp
  authTokenRef: {name: mcp-token, key: token}
`, appsNamespace, appsNamespace, proxyName, testNS, appsNamespace, proxyName, testNS))
		Expect(err).NotTo(HaveOccurred())

		var deployment appsv1.Deployment
		Eventually(func(g Gomega) {
			out, err := kubectl("get", "deployment", proxyName, "-n", testNS, "-o", "json")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(json.Unmarshal([]byte(out), &deployment)).To(Succeed())
			g.Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			g.Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(1))
		}).Should(Succeed())

		pod := &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: testNS},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{{
					Name: podName, Image: curlImage,
					Command: []string{"sh", "-c", `echo "$LITELLM_MCPTOKEN_REMOTE"`},
					Env:     deployment.Spec.Template.Spec.Containers[0].Env,
				}},
			},
		}
		manifest, err := json.Marshal(pod)
		Expect(err).NotTo(HaveOccurred())
		_, err = kubectlApply(string(manifest))
		Expect(err).NotTo(HaveOccurred())
		_, err = kubectl("wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/"+podName,
			"-n", testNS, "--timeout=120s")
		Expect(err).NotTo(HaveOccurred())
		out, err := kubectl("logs", podName, "-n", testNS)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("test-mcp-token\n"))
	})
})

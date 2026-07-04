package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testNS = "default"

var _ = Describe("operational port", func() {
	It("serves metrics and health probes on the single operational port", func() {
		out, err := kubectl("get", "service", release+"-metrics", "-n", namespace,
			"-o", "jsonpath={.spec.ports[0].port}")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal("8081"))

		base := fmt.Sprintf("http://%s-metrics.%s.svc:8081", release, namespace)
		script := fmt.Sprintf("curl -fsS %s/healthz && curl -fsS %s/readyz && curl -fsS %s/metrics", base, base, base)
		defer func() {
			_, _ = kubectl("delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found")
		}()
		// Run detached and read the logs afterwards: `kubectl run -i --rm` can miss
		// the pod's output when the command finishes before attach connects.
		_, err = kubectl("run", "curl-metrics", "-n", namespace, "--restart=Never",
			"--image=curlimages/curl:latest", "--", "sh", "-c", script)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func(g Gomega) {
			phase, err := kubectl("get", "pod", "curl-metrics", "-n", namespace, "-o", "jsonpath={.status.phase}")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(phase).To(Equal("Succeeded"))
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
		out, err = kubectl("logs", "curl-metrics", "-n", namespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("go_goroutines"))
	})
})

var _ = Describe("litellm-operator e2e", Ordered, func() {
	AfterAll(func() {
		_, _ = kubectl("delete", "litellmmodel", "glm", "-n", testNS, "--ignore-not-found")
		_, _ = kubectl("delete", "litellmproxy", "main", "-n", testNS, "--ignore-not-found")
	})

	It("rejects a LiteLLMModel that sets both apiKey and apiKeyRef via the webhook", func() {
		// Retried: right after helm --wait the pod is Ready but the apiserver's
		// route to the webhook Service can lag (kind), yielding an InternalError
		// "connection refused" instead of the denial.
		Eventually(func(g Gomega) {
			out, err := kubectlApply(`
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMModel
metadata:
  name: bad
  namespace: default
spec:
  modelName: bad
  params:
    model: openai/bad
    apiKey: os.environ/FOO
    apiKeyRef:
      name: s
      key: k
`)
			g.Expect(err).To(HaveOccurred())
			g.Expect(out).To(ContainSubstring("mutually exclusive"))
		}, time.Minute, 2*time.Second).Should(Succeed())
	})

	It("reconciles a proxy and its model into a ConfigMap and Deployment", func() {
		_, err := kubectlApply(`
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMProxy
metadata:
  name: main
  namespace: default
spec:
  modelSelector:
    matchLabels:
      proxy: main
---
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMModel
metadata:
  name: glm
  namespace: default
  labels:
    proxy: main
spec:
  modelName: glm-5.2
  params:
    model: openai/glm-5.2
    apiKeyRef:
      name: zai
      key: apikey
`)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			out, err := kubectl("get", "configmap", "main-config", "-n", testNS, "-o", "jsonpath={.data.config\\.yaml}")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring("glm-5.2"))
			g.Expect(out).To(ContainSubstring("os.environ/LITELLM_MODELKEY_GLM"))
		}, 2*time.Minute, 5*time.Second).Should(Succeed())

		Eventually(func(g Gomega) {
			out, err := kubectl("get", "deployment", "main", "-n", testNS,
				"-o", "jsonpath={.spec.template.metadata.annotations.litellm\\.home-operations\\.com/config-hash}")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).NotTo(BeEmpty())
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})
})

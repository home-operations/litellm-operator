package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	apiNS        = "default"
	apiProxyName = "apiproxy"
	masterKey    = "sk-e2e-1234"
)

// postgresManifests stands up a throwaway Postgres for litellm DB mode.
const postgresManifests = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: default
  labels: {app: postgres}
spec:
  replicas: 1
  selector: {matchLabels: {app: postgres}}
  template:
    metadata: {labels: {app: postgres}}
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          env:
            - {name: POSTGRES_USER, value: litellm}
            - {name: POSTGRES_PASSWORD, value: litellm}
            - {name: POSTGRES_DB, value: litellm}
            - {name: PGDATA, value: /tmp/pgdata}
          ports: [{containerPort: 5432}]
          readinessProbe:
            exec: {command: ["pg_isready","-U","litellm"]}
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata: {name: postgres, namespace: default}
spec:
  selector: {app: postgres}
  ports: [{port: 5432, targetPort: 5432}]
`

func apiSecretManifest() string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata: {name: litellm-api, namespace: default}
stringData:
  LITELLM_MASTER_KEY: %q
  DATABASE_URL: postgresql://litellm:litellm@postgres.default.svc:5432/litellm
`, masterKey)
}

const apiProxyManifests = `
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMProxy
metadata: {name: apiproxy, namespace: default}
spec:
  applyMode: api
  image: ghcr.io/berriai/litellm-database:main-stable
  apiAccess:
    masterKeyRef: {name: litellm-api, key: LITELLM_MASTER_KEY}
  modelSelector:
    matchLabels: {proxy: apiproxy}
  envFrom:
    - secretRef: {name: litellm-api}
---
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMModel
metadata:
  name: gpt-4o-mini
  namespace: default
  labels: {proxy: apiproxy}
spec:
  modelName: gpt-4o-mini
  params:
    model: openai/gpt-4o-mini
    apiKey: sk-dummy-not-used-for-registration
`

var _ = Describe("litellm-operator api mode", Ordered, func() {
	AfterAll(func() {
		_, _ = kubectl("delete", "litellmmodel", "gpt-4o-mini", "-n", apiNS, "--ignore-not-found")
		_, _ = kubectl("delete", "litellmproxy", apiProxyName, "-n", apiNS, "--ignore-not-found")
		_, _ = kubectl("delete", "secret", "litellm-api", "-n", apiNS, "--ignore-not-found")
		_, _ = run("kubectl", "delete", "deployment,service", "postgres", "-n", apiNS, "--ignore-not-found")
	})

	It("pushes a model to the proxy's DB-backed admin API", func() {
		By("starting Postgres")
		_, err := kubectlApply(postgresManifests)
		Expect(err).NotTo(HaveOccurred())
		_, err = run("kubectl", "rollout", "status", "deployment/postgres", "-n", apiNS, "--timeout=180s")
		Expect(err).NotTo(HaveOccurred())

		By("creating the master-key + DATABASE_URL secret")
		_, err = kubectlApply(apiSecretManifest())
		Expect(err).NotTo(HaveOccurred())

		By("declaring an api-mode proxy and a model")
		_, err = kubectlApply(apiProxyManifests)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the operator to report the proxy Ready (model synced via API)")
		Eventually(func(g Gomega) {
			out, err := kubectl("get", "litellmproxy", apiProxyName, "-n", apiNS,
				"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("True"))
		}, 9*time.Minute, 10*time.Second).Should(Succeed())

		By("confirming the model is registered on the proxy via the admin API")
		Eventually(func(g Gomega) {
			out, err := run("kubectl", "run", "apicheck", "-n", apiNS,
				"--rm", "--attach", "--restart=Never", "--quiet",
				"--image=curlimages/curl:8.11.1", "--command", "--",
				"curl", "-sS", "-H", "Authorization: Bearer "+masterKey,
				"http://apiproxy.default.svc:4000/model/info")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring("gpt-4o-mini"))
			g.Expect(out).To(ContainSubstring("litellm-operator")) // managed_by tag
		}, 2*time.Minute, 15*time.Second).Should(Succeed())
	})
})

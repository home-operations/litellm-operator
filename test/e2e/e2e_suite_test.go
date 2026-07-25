// Package e2e drives a Helm-installed litellm-operator through its user-facing
// contracts on the Kind cluster $KIND_CLUSTER names (`mise run test-e2e`
// creates and deletes it): the operational port, the validating webhook,
// config-mode reconciliation, and the api-mode admin-API sync.
//
// The suite is excluded from `mise run test` by path, not a build tag, so
// `go vet ./...` still compiles it.
package e2e

import (
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	// Every wait in this suite is a cluster reconcile; specs override only
	// where they need something longer (e.g. pulling the litellm image).
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(5 * time.Second)

	By("building the operator image")
	goVersion := strings.TrimPrefix(runtime.Version(), "go")
	_, err := run("docker", "build", "-t", image, "--build-arg", "GO_VERSION="+goVersion, repoRoot)
	Expect(err).NotTo(HaveOccurred())

	By("loading the image into the kind cluster")
	_, err = run("kind", "load", "docker-image", image, "--name", kindCluster())
	Expect(err).NotTo(HaveOccurred())

	By("installing the chart")
	_, err = run("helm", "upgrade", "--install", release, chartPath,
		"--namespace", namespace, "--create-namespace",
		"--set", "image.repository=litellm-operator",
		"--set", "image.tag=e2e",
		"--set", "image.pullPolicy=Never",
		"--wait", "--timeout", "5m")
	Expect(err).NotTo(HaveOccurred())
})

// The cluster is thrown away right after the run, so dump the state a failure
// needs while it still exists — otherwise CI only shows the failed assertion.
var _ = AfterEach(func() {
	if !CurrentSpecReport().Failed() {
		return
	}
	By("dumping cluster state (spec failed)")
	_, _ = kubectl("get", "pods,litellmproxies,litellmmodels", "-A", "-o", "wide")
	_, _ = kubectl("get", "events", "-A", "--field-selector", "type=Warning", "--sort-by=.lastTimestamp")
	_, _ = kubectl("logs", "-n", namespace,
		"-l", "app.kubernetes.io/name="+release, "--tail=200", "--all-containers")
})

var _ = AfterSuite(func() {
	By("uninstalling the chart")
	_, _ = run("helm", "uninstall", release, "--namespace", namespace, "--ignore-not-found")
})

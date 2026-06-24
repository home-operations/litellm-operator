package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	image     = "litellm-operator:e2e"
	namespace = "litellm-system"
	release   = "litellm-operator"
	chartPath = "../../charts/litellm-operator"
	repoRoot  = "../.."
)

func kindCluster() string {
	if c := os.Getenv("KIND_CLUSTER"); c != "" {
		return c
	}
	return "litellm-operator-e2e"
}

// run executes a command, streaming output to the Ginkgo writer, and returns combined output.
func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	_, _ = fmt.Fprintf(GinkgoWriter, "$ %s %s\n", name, strings.Join(args, " "))
	out, err := cmd.CombinedOutput()
	_, _ = fmt.Fprint(GinkgoWriter, string(out))
	return string(out), err
}

func kubectl(args ...string) (string, error) {
	return run("kubectl", args...)
}

// kubectlApply pipes the given manifest to `kubectl apply -f -`.
func kubectlApply(manifest string) (string, error) {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	_, _ = fmt.Fprint(GinkgoWriter, string(out))
	return string(out), err
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the operator image")
	_, err := run("docker", "build", "-t", image, "--build-arg", "GO_VERSION=1.26.4", repoRoot)
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

var _ = AfterSuite(func() {
	By("uninstalling the chart")
	_, _ = run("helm", "uninstall", release, "--namespace", namespace, "--ignore-not-found")
})

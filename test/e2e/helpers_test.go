package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
)

const (
	image     = "litellm-operator:e2e"
	namespace = "litellm-system"
	release   = "litellm-operator"
	chartPath = "../../charts/litellm-operator"
	repoRoot  = "../.."

	// curlImage is pinned so a moving `latest` can't break the suite.
	curlImage = "curlimages/curl:8.11.1"
)

func kindCluster() string {
	if c := os.Getenv("KIND_CLUSTER"); c != "" {
		return c
	}
	return "litellm-operator-e2e"
}

// run executes a command, streaming output to the Ginkgo writer, and returns combined output.
func run(name string, args ...string) (string, error) {
	GinkgoHelper()
	cmd := exec.Command(name, args...)
	_, _ = fmt.Fprintf(GinkgoWriter, "$ %s %s\n", name, strings.Join(args, " "))
	out, err := cmd.CombinedOutput()
	_, _ = fmt.Fprint(GinkgoWriter, string(out))
	return string(out), err
}

func kubectl(args ...string) (string, error) {
	GinkgoHelper()
	return run("kubectl", args...)
}

// kubectlApply pipes the given manifest to `kubectl apply -f -`.
func kubectlApply(manifest string) (string, error) {
	GinkgoHelper()
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	_, _ = fmt.Fprint(GinkgoWriter, string(out))
	return string(out), err
}

// runPod runs a one-shot pod to completion and returns its logs.
//
// The pod is created detached and its logs are read after it terminates:
// `kubectl run --attach` silently drops the output when the command finishes
// before attach connects, which is the common case for a fast curl.
func runPod(ns, name, img string, timeout time.Duration, command ...string) (string, error) {
	GinkgoHelper()
	_, _ = kubectl("delete", "pod", name, "-n", ns, "--ignore-not-found")
	defer func() { _, _ = kubectl("delete", "pod", name, "-n", ns, "--ignore-not-found") }()

	args := append([]string{
		"run", name, "-n", ns, "--restart=Never", "--image=" + img, "--command", "--",
	}, command...)
	if out, err := kubectl(args...); err != nil {
		return out, fmt.Errorf("create pod %s/%s: %w", ns, name, err)
	}

	_, waitErr := kubectl("wait", "--for=jsonpath={.status.phase}=Succeeded",
		"pod/"+name, "-n", ns, "--timeout="+timeout.String())
	logs, logErr := kubectl("logs", name, "-n", ns)
	if waitErr != nil {
		return logs, fmt.Errorf("pod %s/%s did not succeed: %w (logs: %s)", ns, name, waitErr, logs)
	}
	return logs, logErr
}

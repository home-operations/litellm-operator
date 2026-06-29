package integration

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/controller"
)

// gatewayAPICRDPath resolves the gateway-api module's standard CRD directory
// from the module cache so envtest can serve HTTPRoute.
func gatewayAPICRDPath() string {
	return moduleCRDPath("sigs.k8s.io/gateway-api", "config", "crd", "standard")
}

// llmkubeCRDPath resolves the LLMKube module's CRD directory from the module
// cache so envtest can serve InferenceService and Model.
func llmkubeCRDPath() string {
	return moduleCRDPath("github.com/defilantech/llmkube", "config", "crd", "bases")
}

func moduleCRDPath(module string, sub ...string) string {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", module).Output()
	Expect(err).NotTo(HaveOccurred())
	return filepath.Join(append([]string{strings.TrimSpace(string(out))}, sub...)...)
}

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.Background())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			gatewayAPICRDPath(),
			llmkubeCRDPath(),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(litellmv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(gatewayv1.Install(scheme)).To(Succeed())
	Expect(inferencev1alpha1.AddToScheme(scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"}})
	Expect(err).NotTo(HaveOccurred())

	Expect((&controller.LiteLLMProxyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)).To(Succeed())

	Expect((&controller.LiteLLMMCPServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)).To(Succeed())

	Expect((&controller.LLMKubeInferenceServiceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})

var _ = Describe("LiteLLMProxy reconciliation", func() {
	const (
		ns         = "default"
		proxyName  = "main"
		proxyLabel = "proxy"
		mcpName    = "grafana"
	)

	It("renders matching models into a ConfigMap and rolls the Deployment", func() {
		proxy := &litellmv1alpha1.LiteLLMProxy{
			ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMProxySpec{
				ModelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{proxyLabel: proxyName}},
				Service:       litellmv1alpha1.ProxyServiceSpec{Port: 4000},
				Route: &litellmv1alpha1.ProxyRoute{
					Hostnames:  []string{"litellm.example.com"},
					ParentRefs: []litellmv1alpha1.RouteParentRef{{Name: "envoy", Namespace: "network"}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, proxy)).To(Succeed())

		model := &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: "glm", Namespace: ns, Labels: map[string]string{proxyLabel: proxyName}},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				ModelName: "glm-5.2",
				Params: litellmv1alpha1.LiteLLMParams{
					Model:     "openai/glm-5.2",
					APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "zai", Key: "apikey"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, model)).To(Succeed())

		guardrail := &litellmv1alpha1.LiteLLMGuardrail{
			ObjectMeta: metav1.ObjectMeta{Name: "aporia", Namespace: ns, Labels: map[string]string{proxyLabel: proxyName}},
			Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
				GuardrailName: "aporia-pre", Guardrail: "aporia", Mode: "pre_call",
				APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "gr", Key: "APORIA_KEY"},
			},
		}
		Expect(k8sClient.Create(ctx, guardrail)).To(Succeed())

		mcp := &litellmv1alpha1.LiteLLMMCPServer{
			ObjectMeta: metav1.ObjectMeta{Name: "gh", Namespace: ns, Labels: map[string]string{proxyLabel: proxyName}},
			Spec: litellmv1alpha1.LiteLLMMCPServerSpec{
				Alias: "github", URL: "https://api.githubcopilot.com/mcp", Transport: "http",
			},
		}
		Expect(k8sClient.Create(ctx, mcp)).To(Succeed())

		var cm corev1.ConfigMap
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName + "-config", Namespace: ns}, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("glm-5.2"))
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("os.environ/LITELLM_MODELKEY_GLM"))
			g.Expect(cm.Data["config.yaml"]).NotTo(ContainSubstring("apikey"))
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("aporia-pre"))
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("mcp_servers"))
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("github"))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		var deploy appsv1.Deployment
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName, Namespace: ns}, &deploy)).To(Succeed())
			g.Expect(deploy.Spec.Template.Annotations).To(HaveKey("litellm.home-operations.com/config-hash"))
			envNames := make([]string, 0, len(deploy.Spec.Template.Spec.Containers[0].Env))
			for _, e := range deploy.Spec.Template.Spec.Containers[0].Env {
				envNames = append(envNames, e.Name)
			}
			g.Expect(envNames).To(ContainElement("LITELLM_MODELKEY_GLM"))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		By("creating an HTTPRoute fronting the proxy Service")
		Eventually(func(g Gomega) {
			var route gatewayv1.HTTPRoute
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName, Namespace: ns}, &route)).To(Succeed())
			g.Expect(route.Spec.Hostnames).To(ContainElement(gatewayv1.Hostname("litellm.example.com")))
			g.Expect(route.Spec.Rules[0].BackendRefs[0].Name).To(Equal(gatewayv1.ObjectName(proxyName)))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		hashBefore := deploy.Spec.Template.Annotations["litellm.home-operations.com/config-hash"]

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "glm", Namespace: ns}, model)).To(Succeed())
		model.Spec.Params.Model = "openai/glm-5-turbo"
		Expect(k8sClient.Update(ctx, model)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName, Namespace: ns}, &deploy)).To(Succeed())
			g.Expect(deploy.Spec.Template.Annotations["litellm.home-operations.com/config-hash"]).NotTo(Equal(hashBefore))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("runs a workload-backed MCP server and renders its derived url", func() {
		mcp := &litellmv1alpha1.LiteLLMMCPServer{
			ObjectMeta: metav1.ObjectMeta{Name: mcpName, Namespace: ns, Labels: map[string]string{proxyLabel: proxyName}},
			Spec: litellmv1alpha1.LiteLLMMCPServerSpec{
				Workload: &litellmv1alpha1.MCPWorkloadSpec{Image: "mcp/grafana:latest", Port: 8000},
			},
		}
		Expect(k8sClient.Create(ctx, mcp)).To(Succeed())

		By("creating an owned Deployment and Service for the workload")
		Eventually(func(g Gomega) {
			var deploy appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: mcpName, Namespace: ns}, &deploy)).To(Succeed())
			g.Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal("mcp/grafana:latest"))
			g.Expect(deploy.OwnerReferences).To(HaveLen(1))
			g.Expect(deploy.OwnerReferences[0].Kind).To(Equal("LiteLLMMCPServer"))

			var svc corev1.Service
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: mcpName, Namespace: ns}, &svc)).To(Succeed())
			g.Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8000)))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		By("recording the derived url in status")
		Eventually(func(g Gomega) {
			var got litellmv1alpha1.LiteLLMMCPServer
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: mcpName, Namespace: ns}, &got)).To(Succeed())
			g.Expect(got.Status.ResolvedURL).To(Equal("http://grafana.default.svc.cluster.local:8000/mcp"))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		By("rendering the derived url into the proxy mcp_servers")
		Eventually(func(g Gomega) {
			var cm corev1.ConfigMap
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName + "-config", Namespace: ns}, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("http://grafana.default.svc.cluster.local:8000/mcp"))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})
})

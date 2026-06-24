package integration

import (
	"context"
	"path/filepath"
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

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/controller"
)

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
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
	Expect(litellmv1alpha1.AddToScheme(scheme)).To(Succeed())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"}})
	Expect(err).NotTo(HaveOccurred())

	Expect((&controller.LiteLLMProxyReconciler{
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
		ns        = "default"
		proxyName = "main"
	)

	It("renders matching models into a ConfigMap and rolls the Deployment", func() {
		proxy := &litellmv1alpha1.LiteLLMProxy{
			ObjectMeta: metav1.ObjectMeta{Name: proxyName, Namespace: ns},
			Spec: litellmv1alpha1.LiteLLMProxySpec{
				ModelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"proxy": "main"}},
				Service:       litellmv1alpha1.ProxyServiceSpec{Port: 4000},
			},
		}
		Expect(k8sClient.Create(ctx, proxy)).To(Succeed())

		model := &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: "glm", Namespace: ns, Labels: map[string]string{"proxy": "main"}},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				ModelName: "glm-5.2",
				Params: litellmv1alpha1.LiteLLMParams{
					Model:     "openai/glm-5.2",
					APIKeyRef: &litellmv1alpha1.SecretKeyRef{Name: "zai", Key: "apikey"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, model)).To(Succeed())

		var cm corev1.ConfigMap
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName + "-config", Namespace: ns}, &cm)).To(Succeed())
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("glm-5.2"))
			g.Expect(cm.Data["config.yaml"]).To(ContainSubstring("os.environ/LITELLM_MODELKEY_GLM"))
			g.Expect(cm.Data["config.yaml"]).NotTo(ContainSubstring("apikey"))
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

		hashBefore := deploy.Spec.Template.Annotations["litellm.home-operations.com/config-hash"]

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "glm", Namespace: ns}, model)).To(Succeed())
		model.Spec.Params.Model = "openai/glm-5-turbo"
		Expect(k8sClient.Update(ctx, model)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: proxyName, Namespace: ns}, &deploy)).To(Succeed())
			g.Expect(deploy.Spec.Template.Annotations["litellm.home-operations.com/config-hash"]).NotTo(Equal(hashBefore))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})
})

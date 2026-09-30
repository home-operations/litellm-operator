package integration

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/controller"
)

var _ = Describe("Disabling LLMKube auto-registration", func() {
	It("cleans up existing models after restart and preserves manual models and sources", func(specCtx SpecContext) {
		env := &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases"), llmkubeCRDPath()},
			ErrorIfCRDPathMissing: true,
		}
		cfg, err := env.Start()
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(env.Stop()).To(Succeed()) })

		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).To(Succeed())
		Expect(litellmv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(inferencev1alpha1.AddToScheme(scheme)).To(Succeed())
		c, err := client.New(cfg, client.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())

		start := func(mgr ctrl.Manager) func() {
			managerCtx, cancelManager := context.WithCancel(specCtx)
			group, groupCtx := errgroup.WithContext(managerCtx)
			group.Go(func() error { return mgr.Start(groupCtx) })
			stop := func() {
				cancelManager()
				Expect(group.Wait()).To(Succeed())
			}
			DeferCleanup(stop)
			return stop
		}

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme, Metrics: metricsserver.Options{BindAddress: "0"},
			// This isolated API server shares a process with the suite's registration controller.
			Controller: config.Controller{SkipNameValidation: new(true)},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&controller.LLMKubeInferenceServiceReconciler{
			Client: mgr.GetClient(), Scheme: scheme,
		}).SetupWithManager(mgr)).To(Succeed())
		stopRegistration := start(mgr)

		source := &inferencev1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{Name: "generated", Namespace: "default"},
			Spec:       inferencev1alpha1.InferenceServiceSpec{ModelRef: "llama-ref"},
		}
		Expect(c.Create(specCtx, source)).To(Succeed())
		source.Status.Phase = "Ready"
		source.Status.Endpoint = "http://llama.default.svc.cluster.local:8080/v1"
		Expect(c.Status().Update(specCtx, source)).To(Succeed())
		manual := &litellmv1alpha1.LiteLLMModel{
			ObjectMeta: metav1.ObjectMeta{Name: "manual", Namespace: "default"},
			Spec: litellmv1alpha1.LiteLLMModelSpec{
				ModelName: "manual", Params: litellmv1alpha1.LiteLLMParams{Model: "openai/manual"},
			},
		}
		Expect(c.Create(specCtx, manual)).To(Succeed())
		key := client.ObjectKeyFromObject(source)
		Eventually(func(g Gomega) {
			var model litellmv1alpha1.LiteLLMModel
			g.Expect(c.Get(specCtx, key, &model)).To(Succeed())
			g.Expect(metav1.IsControlledBy(&model, source)).To(BeTrue())
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		stopRegistration()

		By("restarting with cleanup and no LLMKube types registered in its scheme")
		cleanupScheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(cleanupScheme)).To(Succeed())
		Expect(litellmv1alpha1.AddToScheme(cleanupScheme)).To(Succeed())
		cleanupMgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: cleanupScheme, Metrics: metricsserver.Options{BindAddress: "0"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&controller.LLMKubeModelCleanupReconciler{
			Client: cleanupMgr.GetClient(),
		}).SetupWithManager(cleanupMgr)).To(Succeed())
		start(cleanupMgr)
		Eventually(func() bool {
			return apierrors.IsNotFound(c.Get(specCtx, key, &litellmv1alpha1.LiteLLMModel{}))
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
		Expect(c.Get(specCtx, key, &inferencev1alpha1.InferenceService{})).To(Succeed())
		var got litellmv1alpha1.LiteLLMModel
		Expect(c.Get(specCtx, client.ObjectKeyFromObject(manual), &got)).To(Succeed())
		Expect(got.Spec).To(Equal(manual.Spec))
	})
})

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/controller"
	guardrailwebhook "github.com/home-operations/litellm-operator/internal/webhook/litellmguardrail"
	modelwebhook "github.com/home-operations/litellm-operator/internal/webhook/litellmmodel"
	proxywebhook "github.com/home-operations/litellm-operator/internal/webhook/litellmproxy"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	certDir  = "/tmp/k8s-webhook-server/serving-certs"
)

// Populated via -ldflags at build time.
var (
	version = "dev"
	commit  = "unknown"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(litellmv1alpha1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(inferencev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

//nolint:gocyclo
func main() {
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool
	var secureMetrics bool
	var enableHTTP2 bool
	var logLevel string
	var webhookConfigName, webhookServiceName, webhookSecretName string

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service. "+
		"When metrics are served over plain HTTP (--metrics-secure=false), the health/readiness "+
		"probes are co-hosted on this same listener and --health-probe-bind-address is ignored.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Ensures there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level for the controller (debug, info).")
	flag.StringVar(&webhookConfigName, "webhook-config-name", "",
		"Name of the ValidatingWebhookConfiguration to patch with the CA bundle. Empty disables webhooks.")
	flag.StringVar(&webhookServiceName, "webhook-service-name", "",
		"DNS name of the webhook service (e.g. litellm-operator-webhook).")
	flag.StringVar(&webhookSecretName, "webhook-secret-name", "",
		"Name of the Secret holding the webhook serving certificate.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	opts.Development = strings.EqualFold(logLevel, "debug")
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("starting litellm-operator", "version", version, "commit", commit)

	controllerNamespace := os.Getenv("CONTROLLER_NAMESPACE")
	if controllerNamespace == "" {
		controllerNamespace = "litellm-system"
	}
	webhooksEnabled := webhookConfigName != "" && webhookServiceName != "" && webhookSecretName != ""

	// Disabling HTTP/2 mitigates the Stream Cancellation and Rapid Reset CVEs.
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, func(c *tls.Config) { c.NextProtos = []string{"http/1.1"} })
	}

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}
	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	metricsEnabled := metricsAddr != "0"
	coHostHealthOnMetrics := metricsEnabled && !secureMetrics
	healthProbeBindAddress := probeAddr
	if coHostHealthOnMetrics {
		healthProbeBindAddress = "0"
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhook.NewServer(webhook.Options{TLSOpts: tlsOpts}),
		HealthProbeBindAddress: healthProbeBindAddress,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "litellm-operator.home-operations.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.LiteLLMProxyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LiteLLMProxy")
		os.Exit(1)
	}

	if autoRegisterEnabled() {
		setupLLMKubeAutoRegister(mgr)
	}

	certReady := make(chan struct{})
	if webhooksEnabled {
		setupCertRotationAndWebhooks(mgr, certRotationConfig{
			namespace:   controllerNamespace,
			configName:  webhookConfigName,
			serviceName: webhookServiceName,
			secretName:  webhookSecretName,
			ready:       certReady,
		})
	} else {
		close(certReady)
		setupLog.Info("webhooks disabled (set --webhook-config-name/-service-name/-secret-name to enable)")
	}
	// +kubebuilder:scaffold:builder

	if err := setupProbes(mgr, coHostHealthOnMetrics, webhooksEnabled, metricsAddr); err != nil {
		setupLog.Error(err, "unable to set up health checks")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// autoRegisterEnabled reports whether the operator should mirror LLMKube
// InferenceServices into LiteLLMModels. Opt-in via ENABLE_LLMKUBE_AUTOREGISTER
// (any strconv.ParseBool-truthy value: 1, t, true, ...).
func autoRegisterEnabled() bool {
	v := os.Getenv("ENABLE_LLMKUBE_AUTOREGISTER")
	enabled, err := strconv.ParseBool(v)
	return err == nil && enabled
}

// setupLLMKubeAutoRegister wires the InferenceService reconciler, but only if the
// LLMKube CRDs are actually installed. The env var is the user-facing toggle;
// this discovery check is the safety net so enabling the flag on a cluster
// without LLMKube logs a warning and skips rather than crash-looping the whole
// operator (the proxy reconciler shares this process).
func setupLLMKubeAutoRegister(mgr ctrl.Manager) {
	gk := schema.GroupKind{Group: "inference.llmkube.dev", Kind: "InferenceService"}
	if _, err := mgr.GetRESTMapper().RESTMapping(gk, "v1alpha1"); err != nil {
		if meta.IsNoMatchError(err) {
			setupLog.Info("ENABLE_LLMKUBE_AUTOREGISTER is set but the inference.llmkube.dev InferenceService CRD " +
				"is not installed; auto-registration disabled. Install LLMKube and restart the operator to enable it.")
			return
		}
		setupLog.Error(err, "unable to check for LLMKube CRDs; auto-registration disabled")
		return
	}

	if err := (&controller.LLMKubeInferenceServiceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LLMKubeInferenceService")
		os.Exit(1)
	}
	setupLog.Info("LLMKube auto-registration enabled")
}

type certRotationConfig struct {
	namespace   string
	configName  string
	serviceName string
	secretName  string
	ready       chan struct{}
}

func setupCertRotationAndWebhooks(mgr ctrl.Manager, cfg certRotationConfig) {
	dnsName := fmt.Sprintf("%s.%s.svc", cfg.serviceName, cfg.namespace)
	setupLog.Info("setting up cert rotation", "webhook-config", cfg.configName, "dns-name", dnsName)
	if err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey:      types.NamespacedName{Namespace: cfg.namespace, Name: cfg.secretName},
		CertDir:        certDir,
		CAName:         "litellm-operator-ca",
		CAOrganization: "litellm-operator",
		DNSName:        dnsName,
		ExtraDNSNames: []string{
			fmt.Sprintf("%s.%s.svc.cluster.local", cfg.serviceName, cfg.namespace),
		},
		IsReady:              cfg.ready,
		EnableReadinessCheck: true,
		Webhooks:             []rotator.WebhookInfo{{Name: cfg.configName, Type: rotator.Validating}},
	}); err != nil {
		setupLog.Error(err, "unable to set up cert rotation")
		os.Exit(1)
	}

	go func() {
		<-cfg.ready
		setupLog.Info("cert rotation complete, registering webhooks")
		if err := (&modelwebhook.Validator{Client: mgr.GetClient()}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "LiteLLMModel")
			os.Exit(1)
		}
		if err := (&proxywebhook.Validator{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "LiteLLMProxy")
			os.Exit(1)
		}
		if err := (&guardrailwebhook.Validator{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "LiteLLMGuardrail")
			os.Exit(1)
		}
		setupLog.Info("webhooks registered")
	}()
}

func setupProbes(mgr ctrl.Manager, coHostHealthOnMetrics, webhooksEnabled bool, metricsAddr string) error {
	readyz := healthz.Ping
	if webhooksEnabled {
		readyz = func(req *http.Request) error { return mgr.GetWebhookServer().StartedChecker()(req) }
	}

	if coHostHealthOnMetrics {
		if err := mgr.AddMetricsServerExtraHandler("/healthz", healthz.CheckHandler{Checker: healthz.Ping}); err != nil {
			return err
		}
		if err := mgr.AddMetricsServerExtraHandler("/readyz", healthz.CheckHandler{Checker: readyz}); err != nil {
			return err
		}
		setupLog.Info("serving probes on the metrics listener", "bind-address", metricsAddr)
		return nil
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	return mgr.AddReadyzCheck("readyz", readyz)
}

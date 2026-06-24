package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

// LiteLLMProxyReconciler reconciles a LiteLLMProxy and the Deployment, Service,
// and ConfigMap it owns.
type LiteLLMProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmproxies/finalizers,verbs=update
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmmodels;litellmguardrails;litellmmcpservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete

// Reconcile renders the proxy config from matching models and applies the owned resources.
func (r *LiteLLMProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var proxy litellmv1alpha1.LiteLLMProxy
	if err := r.Get(ctx, req.NamespacedName, &proxy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !proxy.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	models, err := r.matchingModels(ctx, &proxy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list models: %w", err)
	}
	guardrails, err := r.matchingGuardrails(ctx, &proxy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list guardrails: %w", err)
	}
	mcpServers, err := r.matchingMCPServers(ctx, &proxy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("list mcp servers: %w", err)
	}

	rendered, err := renderConfig(&proxy, models, guardrails, mcpServers)
	if err != nil {
		return ctrl.Result{}, r.markFailed(ctx, &proxy, "RenderFailed", err.Error())
	}

	if err := r.applyConfigMap(ctx, &proxy, rendered.yaml); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply configmap: %w", err)
	}
	ready, err := r.applyDeployment(ctx, &proxy, rendered)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("apply deployment: %w", err)
	}
	if err := r.applyService(ctx, &proxy); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply service: %w", err)
	}
	if err := r.reconcileRoute(ctx, &proxy); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile route: %w", err)
	}

	logger.Info("reconciled proxy", "models", len(models), "configHash", rendered.hash)
	return ctrl.Result{}, r.markReady(ctx, &proxy, rendered.hash, int32(len(models)), ready)
}

func (r *LiteLLMProxyReconciler) matchingModels(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy) ([]litellmv1alpha1.LiteLLMModel, error) {
	var list litellmv1alpha1.LiteLLMModelList
	if err := r.List(ctx, &list, client.InNamespace(proxy.Namespace)); err != nil {
		return nil, err
	}
	adopted := make([]litellmv1alpha1.LiteLLMModel, 0, len(list.Items))
	for i := range list.Items {
		ok, err := proxyAdopts(proxy, list.Items[i].Spec.ProxyRef, list.Items[i].Labels)
		if err != nil {
			return nil, err
		}
		if ok {
			adopted = append(adopted, list.Items[i])
		}
	}
	return adopted, nil
}

func (r *LiteLLMProxyReconciler) matchingGuardrails(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy) ([]litellmv1alpha1.LiteLLMGuardrail, error) {
	var list litellmv1alpha1.LiteLLMGuardrailList
	if err := r.List(ctx, &list, client.InNamespace(proxy.Namespace)); err != nil {
		return nil, err
	}
	adopted := make([]litellmv1alpha1.LiteLLMGuardrail, 0, len(list.Items))
	for i := range list.Items {
		ok, err := proxyAdopts(proxy, list.Items[i].Spec.ProxyRef, list.Items[i].Labels)
		if err != nil {
			return nil, err
		}
		if ok {
			adopted = append(adopted, list.Items[i])
		}
	}
	return adopted, nil
}

func (r *LiteLLMProxyReconciler) matchingMCPServers(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy) ([]litellmv1alpha1.LiteLLMMCPServer, error) {
	var list litellmv1alpha1.LiteLLMMCPServerList
	if err := r.List(ctx, &list, client.InNamespace(proxy.Namespace)); err != nil {
		return nil, err
	}
	adopted := make([]litellmv1alpha1.LiteLLMMCPServer, 0, len(list.Items))
	for i := range list.Items {
		ok, err := proxyAdopts(proxy, list.Items[i].Spec.ProxyRef, list.Items[i].Labels)
		if err != nil {
			return nil, err
		}
		if ok {
			adopted = append(adopted, list.Items[i])
		}
	}
	return adopted, nil
}

// proxyAdopts reports whether the proxy serves a resource with the given
// proxyRef and labels. An explicit proxyRef wins; otherwise the proxy's
// modelSelector decides, and a proxy with no selector adopts everything in its
// namespace.
func proxyAdopts(proxy *litellmv1alpha1.LiteLLMProxy, proxyRef string, objLabels map[string]string) (bool, error) {
	if proxyRef != "" {
		return proxyRef == proxy.Name, nil
	}
	if proxy.Spec.ModelSelector == nil {
		return true, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(proxy.Spec.ModelSelector)
	if err != nil {
		return false, err
	}
	return selector.Matches(labels.Set(objLabels)), nil
}

func (r *LiteLLMProxyReconciler) applyConfigMap(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy, config string) error {
	desired := buildConfigMap(proxy, config)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Labels = desired.Labels
		cm.Data = desired.Data
		return controllerutil.SetControllerReference(proxy, cm, r.Scheme)
	})
	return err
}

// applyDeployment creates or updates the proxy Deployment and returns its
// current ready replica count. CreateOrUpdate leaves the live status on the
// object, so there is no need to re-Get it (which would race the cache); the
// Owns watch re-reconciles the proxy as the count changes.
func (r *LiteLLMProxyReconciler) applyDeployment(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy, rendered renderedConfig) (int32, error) {
	desired := buildDeployment(proxy, rendered.hash, rendered.envVars)
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Labels = desired.Labels
		deploy.Spec = desired.Spec
		return controllerutil.SetControllerReference(proxy, deploy, r.Scheme)
	}); err != nil {
		return 0, err
	}
	return deploy.Status.ReadyReplicas, nil
}

func (r *LiteLLMProxyReconciler) applyService(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy) error {
	desired := buildService(proxy)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = desired.Labels
		svc.Spec.Type = desired.Spec.Type
		svc.Spec.Selector = desired.Spec.Selector
		svc.Spec.Ports = desired.Spec.Ports
		return controllerutil.SetControllerReference(proxy, svc, r.Scheme)
	})
	return err
}

// reconcileRoute creates/updates the proxy's HTTPRoute when spec.route is set,
// and deletes a previously-created one when it is cleared. The Gateway API CRD
// is optional: when it is absent and no route is requested, the delete's
// no-matching-kind error is ignored so the operator runs on clusters without it.
func (r *LiteLLMProxyReconciler) reconcileRoute(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy) error {
	if proxy.Spec.Route == nil {
		route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: proxy.Name, Namespace: proxy.Namespace}}
		if err := r.Delete(ctx, route); err != nil && !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return err
		}
		return nil
	}
	desired := buildRoute(proxy)
	route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		route.Labels = desired.Labels
		route.Spec = desired.Spec
		return controllerutil.SetControllerReference(proxy, route, r.Scheme)
	})
	return err
}

func (r *LiteLLMProxyReconciler) markReady(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy, hash string, models, ready int32) error {
	meta.SetStatusCondition(&proxy.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            fmt.Sprintf("rendered %d model(s)", models),
		ObservedGeneration: proxy.Generation,
	})
	proxy.Status.ConfigHash = hash
	proxy.Status.ObservedModels = models
	proxy.Status.ReadyReplicas = ready
	return r.Status().Update(ctx, proxy)
}

func (r *LiteLLMProxyReconciler) markFailed(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy, reason, msg string) error {
	meta.SetStatusCondition(&proxy.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: proxy.Generation,
	})
	if err := r.Status().Update(ctx, proxy); err != nil && !apierrors.IsConflict(err) {
		return err
	}
	return fmt.Errorf("%s: %s", reason, msg)
}

// proxiesForObject maps a changed adopted resource (model, guardrail, MCP server)
// to the proxies that serve it. proxyRefOf extracts the object's spec.proxyRef.
func (r *LiteLLMProxyReconciler) proxiesForObject(proxyRefOf func(client.Object) (string, bool)) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		ref, ok := proxyRefOf(obj)
		if !ok {
			return nil
		}
		var proxies litellmv1alpha1.LiteLLMProxyList
		if err := r.List(ctx, &proxies, client.InNamespace(obj.GetNamespace())); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for i := range proxies.Items {
			p := &proxies.Items[i]
			if adopts, err := proxyAdopts(p, ref, obj.GetLabels()); err == nil && adopts {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: p.Name, Namespace: p.Namespace}})
			}
		}
		return requests
	}
}

// SetupWithManager wires the controller and its watches.
func (r *LiteLLMProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	modelRef := func(o client.Object) (string, bool) {
		m, ok := o.(*litellmv1alpha1.LiteLLMModel)
		if !ok {
			return "", false
		}
		return m.Spec.ProxyRef, true
	}
	guardrailRef := func(o client.Object) (string, bool) {
		g, ok := o.(*litellmv1alpha1.LiteLLMGuardrail)
		if !ok {
			return "", false
		}
		return g.Spec.ProxyRef, true
	}
	mcpRef := func(o client.Object) (string, bool) {
		s, ok := o.(*litellmv1alpha1.LiteLLMMCPServer)
		if !ok {
			return "", false
		}
		return s.Spec.ProxyRef, true
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMProxy{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&litellmv1alpha1.LiteLLMModel{}, handler.EnqueueRequestsFromMapFunc(r.proxiesForObject(modelRef))).
		Watches(&litellmv1alpha1.LiteLLMGuardrail{}, handler.EnqueueRequestsFromMapFunc(r.proxiesForObject(guardrailRef))).
		Watches(&litellmv1alpha1.LiteLLMMCPServer{}, handler.EnqueueRequestsFromMapFunc(r.proxiesForObject(mcpRef))).
		Complete(r)
}

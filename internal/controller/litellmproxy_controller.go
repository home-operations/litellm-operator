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
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmmodels,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps,verbs=get;list;watch;create;update;patch;delete

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

	rendered, err := renderConfig(&proxy, models)
	if err != nil {
		return ctrl.Result{}, r.markFailed(ctx, &proxy, "RenderFailed", err.Error())
	}

	if err := r.applyConfigMap(ctx, &proxy, rendered.yaml); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply configmap: %w", err)
	}
	if err := r.applyDeployment(ctx, &proxy, rendered); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply deployment: %w", err)
	}
	if err := r.applyService(ctx, &proxy); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply service: %w", err)
	}

	var deploy appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: proxy.Name, Namespace: proxy.Namespace}, &deploy); err != nil {
		return ctrl.Result{}, fmt.Errorf("get deployment: %w", err)
	}

	logger.Info("reconciled proxy", "models", len(models), "configHash", rendered.hash)
	return ctrl.Result{}, r.markReady(ctx, &proxy, rendered.hash, int32(len(models)), deploy.Status.ReadyReplicas)
}

func (r *LiteLLMProxyReconciler) matchingModels(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy) ([]litellmv1alpha1.LiteLLMModel, error) {
	selector, err := metav1.LabelSelectorAsSelector(&proxy.Spec.ModelSelector)
	if err != nil {
		return nil, err
	}
	var list litellmv1alpha1.LiteLLMModelList
	if err := r.List(ctx, &list,
		client.InNamespace(proxy.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return nil, err
	}
	return list.Items, nil
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

func (r *LiteLLMProxyReconciler) applyDeployment(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy, rendered renderedConfig) error {
	desired := buildDeployment(proxy, rendered.hash, rendered.envVars)
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Labels = desired.Labels
		deploy.Spec = desired.Spec
		return controllerutil.SetControllerReference(proxy, deploy, r.Scheme)
	})
	return err
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

// proxiesForModel maps a changed LiteLLMModel to the proxies whose selector matches it.
func (r *LiteLLMProxyReconciler) proxiesForModel(ctx context.Context, obj client.Object) []reconcile.Request {
	var proxies litellmv1alpha1.LiteLLMProxyList
	if err := r.List(ctx, &proxies, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	modelLabels := labels.Set(obj.GetLabels())
	var requests []reconcile.Request
	for i := range proxies.Items {
		p := &proxies.Items[i]
		selector, err := metav1.LabelSelectorAsSelector(&p.Spec.ModelSelector)
		if err != nil || selector.Empty() {
			continue
		}
		if selector.Matches(modelLabels) {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: p.Name, Namespace: p.Namespace}})
		}
	}
	return requests
}

// SetupWithManager wires the controller and its watches.
func (r *LiteLLMProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMProxy{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&litellmv1alpha1.LiteLLMModel{}, handler.EnqueueRequestsFromMapFunc(r.proxiesForModel)).
		Complete(r)
}

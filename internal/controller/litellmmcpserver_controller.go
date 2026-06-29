package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

// LiteLLMMCPServerReconciler runs the optional workload (Deployment + Service)
// behind a LiteLLMMCPServer and records the url the gateway should use.
type LiteLLMMCPServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmmcpservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmmcpservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile runs the MCP server's workload when spec.workload is set, tears it
// down when it is cleared, and records the resolved url in status.
func (r *LiteLLMMCPServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var server litellmv1alpha1.LiteLLMMCPServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !server.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	if server.Spec.Workload == nil {
		// External-url server: tear down a workload left behind by a cleared
		// spec.workload, then just record the resolved url.
		if err := r.deleteWorkload(ctx, &server); err != nil {
			return ctrl.Result{}, fmt.Errorf("delete mcp workload: %w", err)
		}
		return ctrl.Result{}, r.markReady(ctx, &server)
	}

	if err := r.applyWorkloadDeployment(ctx, &server); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply mcp deployment: %w", err)
	}
	if err := r.applyWorkloadService(ctx, &server); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply mcp service: %w", err)
	}

	logger.Info("reconciled mcp server workload", "name", server.Name, "url", server.ResolvedServerURL())
	return ctrl.Result{}, r.markReady(ctx, &server)
}

func (r *LiteLLMMCPServerReconciler) applyWorkloadDeployment(ctx context.Context, server *litellmv1alpha1.LiteLLMMCPServer) error {
	desired := buildMCPDeployment(server)
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Labels = desired.Labels
		deploy.Spec = desired.Spec
		return controllerutil.SetControllerReference(server, deploy, r.Scheme)
	})
	return err
}

func (r *LiteLLMMCPServerReconciler) applyWorkloadService(ctx context.Context, server *litellmv1alpha1.LiteLLMMCPServer) error {
	desired := buildMCPService(server)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: desired.Name, Namespace: desired.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Labels = desired.Labels
		svc.Spec.Type = desired.Spec.Type
		svc.Spec.Selector = desired.Spec.Selector
		svc.Spec.Ports = desired.Spec.Ports
		return controllerutil.SetControllerReference(server, svc, r.Scheme)
	})
	return err
}

// deleteWorkload removes the Deployment and Service the operator runs for this
// server, but only when it owns them, so an external-url server never deletes an
// unrelated resource that happens to share its name.
func (r *LiteLLMMCPServerReconciler) deleteWorkload(ctx context.Context, server *litellmv1alpha1.LiteLLMMCPServer) error {
	key := types.NamespacedName{Name: server.Name, Namespace: server.Namespace}

	var deploy appsv1.Deployment
	switch err := r.Get(ctx, key, &deploy); {
	case err == nil:
		if metav1.IsControlledBy(&deploy, server) {
			if err := r.Delete(ctx, &deploy); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case !apierrors.IsNotFound(err):
		return err
	}

	var svc corev1.Service
	switch err := r.Get(ctx, key, &svc); {
	case err == nil:
		if metav1.IsControlledBy(&svc, server) {
			if err := r.Delete(ctx, &svc); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case !apierrors.IsNotFound(err):
		return err
	}
	return nil
}

func (r *LiteLLMMCPServerReconciler) markReady(ctx context.Context, server *litellmv1alpha1.LiteLLMMCPServer) error {
	meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "mcp server reconciled",
		ObservedGeneration: server.Generation,
	})
	server.Status.ResolvedURL = server.ResolvedServerURL()
	return r.Status().Update(ctx, server)
}

// SetupWithManager wires the controller and its watches.
func (r *LiteLLMMCPServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMMCPServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

// LLMKubeModelCleanupReconciler removes auto-registered models when registration is disabled.
type LLMKubeModelCleanupReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmmodels,verbs=get;list;watch;delete

// Reconcile deletes models whose labels and controller owner identify an auto-registration.
func (r *LLMKubeModelCleanupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var model litellmv1alpha1.LiteLLMModel
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("llmkube cleanup: get model: %w", err))
	}
	if model.Labels[managedByLabel] != managedByLLMKube || !model.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	owner := metav1.GetControllerOf(&model)
	if owner == nil || owner.APIVersion != inferencev1alpha1.GroupVersion.String() ||
		owner.Kind != "InferenceService" || owner.UID == "" || owner.Name != model.Name ||
		model.Labels[llmkubeServiceLabel] != owner.Name {
		return ctrl.Result{}, nil
	}

	// A stale cache must not delete a model that was replaced or taken over by a user.
	err := r.Delete(ctx, &model, client.Preconditions{UID: &model.UID, ResourceVersion: &model.ResourceVersion})
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("llmkube cleanup: delete model: %w", err))
	}
	return ctrl.Result{}, nil
}

// SetupWithManager watches models without requiring LLMKube CRDs or permissions.
func (r *LLMKubeModelCleanupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMModel{}).
		Named("llmkube-model-cleanup").
		Complete(r)
}

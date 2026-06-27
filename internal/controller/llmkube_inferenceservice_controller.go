package controller

import (
	"context"
	"fmt"
	"maps"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

// inferenceTerminalPhases are the phases where the InferenceService is no longer
// serving and is not expected to recover on its own. Only these trigger removal
// of the generated model; transient phases (Pending/Creating/Progressing/
// WaitingForGPU) leave an existing model in place so a routine pod rollout does
// not churn the proxy ConfigMap and roll the proxy Deployment.
var inferenceTerminalPhases = map[string]bool{
	"Failed":  true,
	"Stopped": true,
}

// LLMKubeInferenceServiceReconciler projects a Ready LLMKube InferenceService
// into a LiteLLMModel in the same namespace. The existing proxy reconciler then
// adopts that model through its normal modelSelector/namespace matching and
// rolls config.yaml — this controller never touches the render path.
//
// The generated model is owned by the InferenceService, so Kubernetes garbage
// collects it when the InferenceService is deleted. Ownership requires the model
// to live in the InferenceService's namespace; this is also where the proxy must
// run to adopt it.
type LLMKubeInferenceServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=inference.llmkube.dev,resources=inferenceservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=inference.llmkube.dev,resources=models,verbs=get;list;watch
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmmodels,verbs=get;list;watch;create;update;patch;delete

// Reconcile keeps the generated LiteLLMModel in sync with the InferenceService.
func (r *LLMKubeInferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var isvc inferencev1alpha1.InferenceService
	if err := r.Get(ctx, req.NamespacedName, &isvc); err != nil {
		// Gone: the owned LiteLLMModel is garbage collected by Kubernetes.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !isvc.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	key := types.NamespacedName{Name: isvc.Name, Namespace: isvc.Namespace}

	if !inferenceServiceReady(&isvc) {
		// Only tear the model down on a terminal phase; ride out transient dips.
		if inferenceTerminalPhases[isvc.Status.Phase] {
			return ctrl.Result{}, r.deleteManagedModel(ctx, key)
		}
		return ctrl.Result{}, nil
	}

	// Best-effort capability metadata: a missing Model just omits model_info.
	var model *inferencev1alpha1.Model
	if isvc.Spec.ModelRef != "" {
		var fetched inferencev1alpha1.Model
		err := r.Get(ctx, types.NamespacedName{Name: isvc.Spec.ModelRef, Namespace: isvc.Namespace}, &fetched)
		switch {
		case err == nil:
			model = &fetched
		case !apierrors.IsNotFound(err):
			return ctrl.Result{}, fmt.Errorf("get model %q: %w", isvc.Spec.ModelRef, err)
		}
	}

	desired := projectInferenceService(&isvc, model)
	return ctrl.Result{}, r.applyManagedModel(ctx, &isvc, desired)
}

// applyManagedModel creates or updates the generated model, refusing to touch a
// pre-existing model that the operator does not own.
func (r *LLMKubeInferenceServiceReconciler) applyManagedModel(
	ctx context.Context,
	isvc *inferencev1alpha1.InferenceService,
	desired *litellmv1alpha1.LiteLLMModel,
) error {
	logger := log.FromContext(ctx)
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}

	var existing litellmv1alpha1.LiteLLMModel
	err := r.Get(ctx, key, &existing)
	switch {
	case apierrors.IsNotFound(err):
		if err := controllerutil.SetControllerReference(isvc, desired, r.Scheme); err != nil {
			return err
		}
		if err := r.Create(ctx, desired); err != nil {
			// A concurrent reconcile already created it; the Owns watch will
			// re-enqueue us to converge on the update path.
			return client.IgnoreAlreadyExists(err)
		}
		logger.Info("registered model from InferenceService", "model", key.Name)
		return nil
	case err != nil:
		return err
	}

	if existing.Labels[managedByLabel] != managedByLLMKube {
		logger.Info("LiteLLMModel exists and is not operator-managed; skipping to avoid overwrite",
			"model", key.Name)
		return nil
	}

	// Merge the managed labels so user-added labels (e.g. an extra proxy
	// selector) survive; the spec is fully operator-owned and replaced.
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	maps.Copy(existing.Labels, desired.Labels)
	existing.Spec = desired.Spec
	if err := controllerutil.SetControllerReference(isvc, &existing, r.Scheme); err != nil {
		return err
	}
	return r.Update(ctx, &existing)
}

// deleteManagedModel removes the generated model, leaving any same-named model
// the operator does not own untouched.
func (r *LLMKubeInferenceServiceReconciler) deleteManagedModel(ctx context.Context, key types.NamespacedName) error {
	var existing litellmv1alpha1.LiteLLMModel
	if err := r.Get(ctx, key, &existing); err != nil {
		return client.IgnoreNotFound(err)
	}
	if existing.Labels[managedByLabel] != managedByLLMKube {
		return nil
	}
	return client.IgnoreNotFound(r.Delete(ctx, &existing))
}

// SetupWithManager wires the controller. Owns(LiteLLMModel) drives both garbage
// collection on InferenceService delete and self-healing if the model is removed
// out of band. The proxy adopts the model via its own watch, so no LiteLLMProxy
// watch is needed here.
func (r *LLMKubeInferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&inferencev1alpha1.InferenceService{}).
		Owns(&litellmv1alpha1.LiteLLMModel{}).
		Named("llmkube-inferenceservice").
		Complete(r)
}

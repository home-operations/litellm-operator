package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	inferencev1alpha1 "github.com/defilantech/llmkube/api/v1alpha1"
	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

func cleanupModel() *litellmv1alpha1.LiteLLMModel {
	model := projectInferenceService(readyISVC("llama", "llama-ref", chatEndpoint), nil)
	model.UID = "model-uid"
	model.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: inferencev1alpha1.GroupVersion.String(),
		Kind:       "InferenceService", Name: model.Name, UID: "service-uid", Controller: new(true),
	}}
	return model
}

func TestLLMKubeModelCleanup(t *testing.T) {
	tests := []struct {
		name   string
		change func(*litellmv1alpha1.LiteLLMModel)
		remove bool
	}{
		{name: "generated model", remove: true},
		{name: "manual model", change: func(m *litellmv1alpha1.LiteLLMModel) {
			m.Labels = nil
			m.OwnerReferences = nil
		}},
		{name: "labels without ownership", change: func(m *litellmv1alpha1.LiteLLMModel) { m.OwnerReferences = nil }},
		{name: "ownership without labels", change: func(m *litellmv1alpha1.LiteLLMModel) { m.Labels = nil }},
		{name: "other manager", change: func(m *litellmv1alpha1.LiteLLMModel) { m.Labels[managedByLabel] = "user" }},
		{name: "other API group", change: func(m *litellmv1alpha1.LiteLLMModel) { m.OwnerReferences[0].APIVersion = "example.com/v1" }},
		{name: "other kind", change: func(m *litellmv1alpha1.LiteLLMModel) { m.OwnerReferences[0].Kind = "Model" }},
		{name: "non-controller owner", change: func(m *litellmv1alpha1.LiteLLMModel) { m.OwnerReferences[0].Controller = new(false) }},
		{name: "different owner name", change: func(m *litellmv1alpha1.LiteLLMModel) { m.OwnerReferences[0].Name = "other" }},
		{name: "missing owner UID", change: func(m *litellmv1alpha1.LiteLLMModel) { m.OwnerReferences[0].UID = "" }},
		{name: "different source label", change: func(m *litellmv1alpha1.LiteLLMModel) { m.Labels[llmkubeServiceLabel] = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			model := cleanupModel()
			if tt.change != nil {
				tt.change(model)
			}
			scheme := runtime.NewScheme()
			require.NoError(t, litellmv1alpha1.AddToScheme(scheme))
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(model).Build()
			r := &LLMKubeModelCleanupReconciler{Client: c}
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}
			_, err := r.Reconcile(ctx, req)
			require.NoError(t, err)
			var got litellmv1alpha1.LiteLLMModel
			err = c.Get(ctx, req.NamespacedName, &got)
			if tt.remove {
				require.True(t, apierrors.IsNotFound(err), "generated model must be deleted")
			} else {
				require.NoError(t, err)
				require.Equal(t, model.Spec, got.Spec)
				require.Equal(t, model.Labels, got.Labels)
				require.Equal(t, model.OwnerReferences, got.OwnerReferences)
			}
			_, err = r.Reconcile(ctx, req)
			require.NoError(t, err)
		})
	}
}

func TestLLMKubeModelCleanupConcurrentTakeover(t *testing.T) {
	ctx := t.Context()
	model := cleanupModel()
	scheme := runtime.NewScheme()
	require.NoError(t, litellmv1alpha1.AddToScheme(scheme))
	deleteCalls := 0
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(model).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				deleteCalls++
				options := (&client.DeleteOptions{}).ApplyOptions(opts)
				require.NotNil(t, options.Preconditions)
				require.Equal(t, obj.GetUID(), *options.Preconditions.UID)
				require.Equal(t, obj.GetResourceVersion(), *options.Preconditions.ResourceVersion)
				var manual litellmv1alpha1.LiteLLMModel
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), &manual))
				manual.Labels = nil
				manual.OwnerReferences = nil
				require.NoError(t, c.Update(ctx, &manual))
				return c.Delete(ctx, obj, opts...)
			},
		}).Build()
	r := &LLMKubeModelCleanupReconciler{Client: c}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)}
	_, err := r.Reconcile(ctx, req)
	require.True(t, apierrors.IsConflict(err), "a stale read must not delete the converted model")
	_, err = r.Reconcile(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, deleteCalls)
	var got litellmv1alpha1.LiteLLMModel
	require.NoError(t, c.Get(ctx, req.NamespacedName, &got))
	require.Empty(t, got.Labels)
	require.Empty(t, got.OwnerReferences)
}

func TestLLMKubeModelCleanupErrors(t *testing.T) {
	failed := errors.New("API unavailable")
	for _, operation := range []string{"get", "delete"} {
		t.Run(operation, func(t *testing.T) {
			model := cleanupModel()
			scheme := runtime.NewScheme()
			require.NoError(t, litellmv1alpha1.AddToScheme(scheme))
			funcs := interceptor.Funcs{}
			if operation == "get" {
				funcs.Get = func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					return failed
				}
			} else {
				funcs.Delete = func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
					return failed
				}
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(model).WithInterceptorFuncs(funcs).Build()
			r := &LLMKubeModelCleanupReconciler{Client: c}
			_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(model)})
			require.ErrorIs(t, err, failed)
		})
	}
}

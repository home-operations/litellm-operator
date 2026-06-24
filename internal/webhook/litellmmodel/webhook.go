package litellmmodel

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var modellog = logf.Log.WithName("litellmmodel-resource")

// Validator validates LiteLLMModel resources at admission.
type Validator struct {
	Client client.Client
}

// +kubebuilder:webhook:path=/validate-litellm-home-operations-com-v1alpha1-litellmmodel,mutating=false,failurePolicy=fail,sideEffects=None,groups=litellm.home-operations.com,resources=litellmmodels,verbs=create;update,versions=v1alpha1,name=vlitellmmodel.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*litellmv1alpha1.LiteLLMModel] = &Validator{}

// ValidateCreate validates a new LiteLLMModel.
func (v *Validator) ValidateCreate(ctx context.Context, m *litellmv1alpha1.LiteLLMModel) (admission.Warnings, error) {
	modellog.Info("validate create", "name", m.Name, "namespace", m.Namespace)
	return v.validate(ctx, m)
}

// ValidateUpdate validates an updated LiteLLMModel.
func (v *Validator) ValidateUpdate(ctx context.Context, _, m *litellmv1alpha1.LiteLLMModel) (admission.Warnings, error) {
	modellog.Info("validate update", "name", m.Name, "namespace", m.Namespace)
	return v.validate(ctx, m)
}

// ValidateDelete is a no-op; models are freely deletable.
func (v *Validator) ValidateDelete(context.Context, *litellmv1alpha1.LiteLLMModel) (admission.Warnings, error) {
	return nil, nil
}

func (v *Validator) validate(ctx context.Context, m *litellmv1alpha1.LiteLLMModel) (admission.Warnings, error) {
	p := m.Spec.Params
	if p.APIKeyRef != nil && p.APIKey != "" {
		return nil, fmt.Errorf("spec.params.apiKey and spec.params.apiKeyRef are mutually exclusive; set only one")
	}
	if p.APIBaseRef != nil && p.APIBase != "" {
		return nil, fmt.Errorf("spec.params.apiBase and spec.params.apiBaseRef are mutually exclusive; set only one")
	}
	if p.APIKeyRef == nil {
		return nil, nil
	}

	envName := m.APIKeyEnvVarName()
	var list litellmv1alpha1.LiteLLMModelList
	if err := v.Client.List(ctx, &list, client.InNamespace(m.Namespace)); err != nil {
		return nil, fmt.Errorf("list models in namespace %q: %w", m.Namespace, err)
	}
	for i := range list.Items {
		other := &list.Items[i]
		if other.Name == m.Name || other.Spec.Params.APIKeyRef == nil {
			continue
		}
		if other.APIKeyEnvVarName() == envName {
			return nil, fmt.Errorf(
				"model %q derives the same API key env var %q as model %q; rename one so their sanitized names differ",
				m.Name, envName, other.Name)
		}
	}
	return nil, nil
}

// SetupWebhookWithManager registers the validating webhook.
func (v *Validator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &litellmv1alpha1.LiteLLMModel{}).
		WithValidator(v).
		Complete()
}

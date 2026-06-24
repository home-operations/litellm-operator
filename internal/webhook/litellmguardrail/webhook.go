package litellmguardrail

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var guardraillog = logf.Log.WithName("litellmguardrail-resource")

// Validator validates LiteLLMGuardrail resources at admission.
type Validator struct{}

// +kubebuilder:webhook:path=/validate-litellm-home-operations-com-v1alpha1-litellmguardrail,mutating=false,failurePolicy=fail,sideEffects=None,groups=litellm.home-operations.com,resources=litellmguardrails,verbs=create;update,versions=v1alpha1,name=vlitellmguardrail.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*litellmv1alpha1.LiteLLMGuardrail] = &Validator{}

// ValidateCreate validates a new LiteLLMGuardrail.
func (v *Validator) ValidateCreate(_ context.Context, g *litellmv1alpha1.LiteLLMGuardrail) (admission.Warnings, error) {
	guardraillog.Info("validate create", "name", g.Name, "namespace", g.Namespace)
	return v.validate(g)
}

// ValidateUpdate validates an updated LiteLLMGuardrail.
func (v *Validator) ValidateUpdate(_ context.Context, _, g *litellmv1alpha1.LiteLLMGuardrail) (admission.Warnings, error) {
	guardraillog.Info("validate update", "name", g.Name, "namespace", g.Namespace)
	return v.validate(g)
}

// ValidateDelete is a no-op; guardrails are freely deletable.
func (v *Validator) ValidateDelete(context.Context, *litellmv1alpha1.LiteLLMGuardrail) (admission.Warnings, error) {
	return nil, nil
}

func (v *Validator) validate(g *litellmv1alpha1.LiteLLMGuardrail) (admission.Warnings, error) {
	if g.Spec.APIKeyRef != nil && g.Spec.APIKey != "" {
		return nil, fmt.Errorf("spec.apiKey and spec.apiKeyRef are mutually exclusive; set only one")
	}
	if g.Spec.APIBaseRef != nil && g.Spec.APIBase != "" {
		return nil, fmt.Errorf("spec.apiBase and spec.apiBaseRef are mutually exclusive; set only one")
	}
	return nil, nil
}

// SetupWebhookWithManager registers the validating webhook.
func (v *Validator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &litellmv1alpha1.LiteLLMGuardrail{}).
		WithValidator(v).
		Complete()
}

package litellmmcpserver

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var mcplog = logf.Log.WithName("litellmmcpserver-resource")

// Validator validates LiteLLMMCPServer resources at admission.
type Validator struct{}

// +kubebuilder:webhook:path=/validate-litellm-home-operations-com-v1alpha1-litellmmcpserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=litellm.home-operations.com,resources=litellmmcpservers,verbs=create;update,versions=v1alpha1,name=vlitellmmcpserver.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*litellmv1alpha1.LiteLLMMCPServer] = &Validator{}

// ValidateCreate validates a new LiteLLMMCPServer.
func (v *Validator) ValidateCreate(_ context.Context, s *litellmv1alpha1.LiteLLMMCPServer) (admission.Warnings, error) {
	mcplog.Info("validate create", "name", s.Name, "namespace", s.Namespace)
	return nil, validate(s)
}

// ValidateUpdate validates an updated LiteLLMMCPServer.
func (v *Validator) ValidateUpdate(_ context.Context, _, s *litellmv1alpha1.LiteLLMMCPServer) (admission.Warnings, error) {
	mcplog.Info("validate update", "name", s.Name, "namespace", s.Namespace)
	return nil, validate(s)
}

// ValidateDelete is a no-op; MCP servers are freely deletable.
func (v *Validator) ValidateDelete(context.Context, *litellmv1alpha1.LiteLLMMCPServer) (admission.Warnings, error) {
	return nil, nil
}

// validate enforces that exactly one of spec.url or spec.workload is set.
func validate(s *litellmv1alpha1.LiteLLMMCPServer) error {
	hasURL := s.Spec.URL != ""
	hasWorkload := s.Spec.Workload != nil
	switch {
	case hasURL && hasWorkload:
		return fmt.Errorf("spec.url and spec.workload are mutually exclusive; set only one")
	case !hasURL && !hasWorkload:
		return fmt.Errorf("one of spec.url or spec.workload is required")
	}
	return nil
}

// SetupWebhookWithManager registers the validating webhook.
func (v *Validator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &litellmv1alpha1.LiteLLMMCPServer{}).
		WithValidator(v).
		Complete()
}

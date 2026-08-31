package litellmproxy

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var proxylog = logf.Log.WithName("litellmproxy-resource")

// Validator validates LiteLLMProxy resources at admission.
type Validator struct{}

// +kubebuilder:webhook:path=/validate-litellm-home-operations-com-v1alpha1-litellmproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=litellm.home-operations.com,resources=litellmproxies,verbs=create;update,versions=v1alpha1,name=vlitellmproxy.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*litellmv1alpha1.LiteLLMProxy] = &Validator{}

// ValidateCreate validates a new LiteLLMProxy.
func (v *Validator) ValidateCreate(_ context.Context, p *litellmv1alpha1.LiteLLMProxy) (admission.Warnings, error) {
	proxylog.Info("validate create", "name", p.Name, "namespace", p.Namespace)
	return v.validate(p)
}

// ValidateUpdate validates an updated LiteLLMProxy.
func (v *Validator) ValidateUpdate(_ context.Context, _, p *litellmv1alpha1.LiteLLMProxy) (admission.Warnings, error) {
	proxylog.Info("validate update", "name", p.Name, "namespace", p.Namespace)
	return v.validate(p)
}

// ValidateDelete is a no-op; proxies are freely deletable.
func (v *Validator) ValidateDelete(context.Context, *litellmv1alpha1.LiteLLMProxy) (admission.Warnings, error) {
	return nil, nil
}

func (v *Validator) validate(p *litellmv1alpha1.LiteLLMProxy) (admission.Warnings, error) {
	if p.Spec.ApplyMode == "api" {
		if p.Spec.APIAccess == nil || p.Spec.APIAccess.MasterKeyRef.Name == "" {
			return nil, fmt.Errorf("spec.applyMode=api requires spec.apiAccess.masterKeyRef")
		}
	}

	for i, v := range p.Spec.Volumes {
		if v.Name == litellmv1alpha1.ProxyConfigVolumeName {
			return nil, fmt.Errorf("spec.volumes[%d].name %q is reserved for the operator-managed config volume", i, v.Name)
		}
	}
	for i, m := range p.Spec.VolumeMounts {
		if m.Name == litellmv1alpha1.ProxyConfigVolumeName {
			return nil, fmt.Errorf("spec.volumeMounts[%d].name %q is reserved for the operator-managed config mount", i, m.Name)
		}
		if m.MountPath == litellmv1alpha1.ProxyConfigMountPath {
			return nil, fmt.Errorf("spec.volumeMounts[%d].mountPath %q is reserved for the operator-managed config mount", i, m.MountPath)
		}
	}

	route := p.Spec.Route
	if route == nil {
		return nil, nil
	}
	if len(route.Hostnames) == 0 {
		return nil, fmt.Errorf("spec.route.hostnames must not be empty")
	}
	if len(route.ParentRefs) == 0 {
		return nil, fmt.Errorf("spec.route.parentRefs must not be empty")
	}
	for i, ref := range route.ParentRefs {
		if ref.Name == "" {
			return nil, fmt.Errorf("spec.route.parentRefs[%d].name must not be empty", i)
		}
		// The apiserver defaults an unset group to gateway.networking.k8s.io, which
		// has no Service kind; the HTTPRoute would be accepted and never attach.
		if ref.Kind != nil && *ref.Kind == "Service" && (ref.Group == nil || *ref.Group == gatewayv1.GroupName) {
			return nil, fmt.Errorf("spec.route.parentRefs[%d].group must be explicitly set to \"\" for kind Service", i)
		}
		for j := range i {
			if !sameParent(route.ParentRefs[j], ref) {
				continue
			}
			// Mirrors the standard-channel HTTPRoute rule, which does not tell refs
			// to the same parent apart by port.
			if ref.SectionName == nil || route.ParentRefs[j].SectionName == nil {
				return nil, fmt.Errorf("spec.route.parentRefs[%d] and [%d] reference the same parent; each must set a distinct sectionName", j, i)
			}
			if *ref.SectionName == *route.ParentRefs[j].SectionName {
				return nil, fmt.Errorf("spec.route.parentRefs[%d] and [%d] reference the same parent and sectionName", j, i)
			}
		}
	}
	return nil, nil
}

// sameParent reports whether two refs name the same parent object, applying
// the Gateway API defaults for an unset group or kind.
func sameParent(a, b gatewayv1.ParentReference) bool {
	return ptrOr(a.Group, gatewayv1.GroupName) == ptrOr(b.Group, gatewayv1.GroupName) &&
		ptrOr(a.Kind, "Gateway") == ptrOr(b.Kind, "Gateway") &&
		ptrOr(a.Namespace, "") == ptrOr(b.Namespace, "") &&
		a.Name == b.Name
}

func ptrOr[T ~string](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// SetupWebhookWithManager registers the validating webhook.
func (v *Validator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &litellmv1alpha1.LiteLLMProxy{}).
		WithValidator(v).
		Complete()
}

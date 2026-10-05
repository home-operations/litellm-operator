package controller

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const credentialsHashAnnotation = "litellm.home-operations.com/credentials-hash"

func (r *LiteLLMProxyReconciler) reconcileCredentials(ctx context.Context, proxy *litellmv1alpha1.LiteLLMProxy, envVars []corev1.EnvVar, owners map[string]types.NamespacedName) (string, error) {
	credentials := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: proxy.Name + "-credentials", Namespace: proxy.Namespace,
	}}
	data := map[string][]byte{}
	for i := range envVars {
		env := &envVars[i]
		namespace := owners[env.Name].Namespace
		if namespace == proxy.Namespace {
			continue
		}
		ref := env.ValueFrom.SecretKeyRef
		value, err := readSecretKey(ctx, r.Client, namespace, litellmv1alpha1.SecretKeyRef{Name: ref.Name, Key: ref.Key})
		if err != nil {
			return "", fmt.Errorf("resolve %s from secret %s/%s: %w", env.Name, namespace, ref.Name, err)
		}
		data[env.Name] = []byte(value)
		ref.Name = credentials.Name
		ref.Key = env.Name
	}
	if len(data) == 0 {
		if err := r.Get(ctx, client.ObjectKeyFromObject(credentials), credentials); err != nil {
			return "", client.IgnoreNotFound(err)
		}
		if metav1.IsControlledBy(credentials, proxy) {
			return "", client.IgnoreNotFound(r.Delete(ctx, credentials))
		}
		return "", nil
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, credentials, func() error {
		if credentials.ResourceVersion != "" && !metav1.IsControlledBy(credentials, proxy) {
			return fmt.Errorf("secret %s/%s is not owned by this proxy", credentials.Namespace, credentials.Name)
		}
		credentials.Labels = selectorLabels(proxy)
		credentials.Type = corev1.SecretTypeOpaque
		credentials.Data = data
		return controllerutil.SetControllerReference(proxy, credentials, r.Scheme)
	})
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return hashString(string(encoded)), nil
}

func referencesSecret(name string, refs ...*litellmv1alpha1.SecretKeyRef) bool {
	for _, ref := range refs {
		if ref != nil && ref.Name == name {
			return true
		}
	}
	return false
}

func (r *LiteLLMProxyReconciler) proxiesForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	var models litellmv1alpha1.LiteLLMModelList
	var guardrails litellmv1alpha1.LiteLLMGuardrailList
	var servers litellmv1alpha1.LiteLLMMCPServerList
	for _, list := range []client.ObjectList{&models, &guardrails, &servers} {
		if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
			log.FromContext(ctx).Error(err, "list resources referencing secret", "namespace", obj.GetNamespace())
			return nil
		}
	}
	requests := map[reconcile.Request]struct{}{}
	enqueue := func(resource client.Object) {
		for _, req := range r.proxiesForObject(ctx, resource) {
			requests[req] = struct{}{}
		}
	}
	for i := range models.Items {
		m := &models.Items[i]
		if referencesSecret(obj.GetName(), m.Spec.Params.APIKeyRef, m.Spec.Params.APIBaseRef) {
			enqueue(m)
		}
	}
	for i := range guardrails.Items {
		g := &guardrails.Items[i]
		if referencesSecret(obj.GetName(), g.Spec.APIKeyRef, g.Spec.APIBaseRef) {
			enqueue(g)
		}
	}
	for i := range servers.Items {
		s := &servers.Items[i]
		if referencesSecret(obj.GetName(), s.Spec.AuthTokenRef) {
			enqueue(s)
		}
	}
	result := make([]reconcile.Request, 0, len(requests))
	for req := range requests {
		result = append(result, req)
	}
	return result
}

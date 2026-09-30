package controller

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/litellmclient"
)

const virtualKeyFinalizer = "litellm.home-operations.com/virtual-key"

// Keys the operator stamped on the output Secret on the previous reconcile,
// recorded as sorted comma-separated lists so a key dropped from the spec can be
// removed without disturbing metadata any other controller owns.
const (
	managedAnnotationKeysAnnotation = "litellm.home-operations.com/managed-annotations"
	managedLabelKeysAnnotation      = "litellm.home-operations.com/managed-labels"

	// Prefix the bookkeeping annotations share. The spec may not declare keys under
	// it: recordManaged writes them last, so a collision would silently discard the
	// user's value on every reconcile.
	managedKeysPrefix         = "litellm.home-operations.com/managed-"
	managedDurationAnnotation = managedKeysPrefix + "duration"
)

// LiteLLMVirtualKeyReconciler creates LiteLLM virtual keys and stores them in
// Secrets owned by their LiteLLMVirtualKey resources.
type LiteLLMVirtualKeyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmvirtualkeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmvirtualkeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmvirtualkeys/finalizers,verbs=update
// +kubebuilder:rbac:groups=litellm.home-operations.com,resources=litellmproxies,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile updates key settings and deletes the key when the resource is removed.
func (r *LiteLLMVirtualKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var virtualKey litellmv1alpha1.LiteLLMVirtualKey
	if err := r.Get(ctx, req.NamespacedName, &virtualKey); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !virtualKey.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, &virtualKey)
	}
	if !controllerutil.ContainsFinalizer(&virtualKey, virtualKeyFinalizer) {
		controllerutil.AddFinalizer(&virtualKey, virtualKeyFinalizer)
		if err := r.Update(ctx, &virtualKey); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
	}

	// Guarded ahead of both the Secret-exists and Secret-absent paths below, so
	// neither writes Secret metadata nor calls the admin API for an invalid spec.
	if err := reservedSecretMetadata(&virtualKey); err != nil {
		return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "InvalidSpec", err.Error())
	}

	secret, err := r.outputSecret(ctx, &virtualKey)
	switch {
	case err == nil:
		if !metav1.IsControlledBy(secret, &virtualKey) {
			return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "SecretNotOwned", "output Secret is not owned by this resource")
		}
		if len(secret.Data[virtualKey.SecretDataKey()]) == 0 {
			return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "SecretKeyMissing", "output Secret does not contain the generated key")
		}
		if applySecretMetadata(secret, &virtualKey) {
			if err := r.Update(ctx, secret); err != nil {
				return ctrl.Result{}, fmt.Errorf("update output Secret metadata: %w", err)
			}
		}
		admin, err := r.adminClient(ctx, &virtualKey)
		if err != nil {
			return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "AdminClientFailed", err.Error())
		}
		requestBody, err := virtualKeyRequest(&virtualKey)
		if err != nil {
			return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "InvalidSpec", err.Error())
		}
		live, err := admin.GetVirtualKey(ctx, string(secret.Data[virtualKey.SecretDataKey()]))
		if err != nil {
			return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "GetFailed", err.Error())
		}
		// LiteLLM returns expires rather than the duration used to calculate it.
		updateDuration := secret.Annotations[managedDurationAnnotation] != requestBody.Duration ||
			(live.Expires == nil) != (requestBody.Duration == "")
		if virtualKeyRequestsEqual(live, requestBody) && !updateDuration {
			return ctrl.Result{}, r.markReady(ctx, &virtualKey)
		}
		if err := admin.UpdateVirtualKey(ctx, string(secret.Data[virtualKey.SecretDataKey()]), requestBody, updateDuration); err != nil {
			return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "UpdateFailed", err.Error())
		}
		if updateDuration {
			if secret.Annotations == nil {
				secret.Annotations = make(map[string]string)
			}
			secret.Annotations[managedDurationAnnotation] = requestBody.Duration
			if err := r.Update(ctx, secret); err != nil {
				return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "SecretUpdateFailed", err.Error())
			}
		}
		return ctrl.Result{}, r.markReady(ctx, &virtualKey)
	case !apierrors.IsNotFound(err):
		return ctrl.Result{}, fmt.Errorf("get output Secret: %w", err)
	}

	admin, err := r.adminClient(ctx, &virtualKey)
	if err != nil {
		return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "AdminClientFailed", err.Error())
	}
	requestBody, err := virtualKeyRequest(&virtualKey)
	if err != nil {
		return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "InvalidSpec", err.Error())
	}
	generated, err := admin.GenerateVirtualKey(ctx, requestBody)
	if err != nil {
		return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "GenerateFailed", err.Error())
	}
	if generated.Key == "" {
		return ctrl.Result{}, r.markFailed(ctx, &virtualKey, "GenerateFailed", "LiteLLM returned an empty key")
	}

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: virtualKey.Spec.SecretName, Namespace: virtualKey.Namespace},
		Data:       map[string][]byte{virtualKey.SecretDataKey(): []byte(generated.Key)},
	}
	applySecretMetadata(secret, &virtualKey)
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[managedDurationAnnotation] = virtualKey.Spec.Duration
	if err := controllerutil.SetControllerReference(&virtualKey, secret, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("set output Secret owner: %w", err)
	}
	if err := r.Create(ctx, secret); err != nil {
		if deleteErr := admin.DeleteVirtualKey(ctx, generated.Key); deleteErr != nil {
			return ctrl.Result{}, fmt.Errorf("create output Secret: %w (delete generated key: %v)", err, deleteErr)
		}
		return ctrl.Result{}, fmt.Errorf("create output Secret: %w", err)
	}
	return ctrl.Result{}, r.markReady(ctx, &virtualKey)
}

// reservedSecretMetadata reports the lowest-sorting Secret annotation or label key
// the spec declares inside the operator's bookkeeping namespace, so the message
// names the same key on every reconcile.
func reservedSecretMetadata(virtualKey *litellmv1alpha1.LiteLLMVirtualKey) error {
	for _, field := range []struct {
		name    string
		desired map[string]string
	}{
		{"secretAnnotations", virtualKey.Spec.SecretAnnotations},
		{"secretLabels", virtualKey.Spec.SecretLabels},
	} {
		for _, key := range slices.Sorted(maps.Keys(field.desired)) {
			if strings.HasPrefix(key, managedKeysPrefix) {
				return fmt.Errorf("spec.%s key %q is reserved: the operator manages keys prefixed %q", field.name, key, managedKeysPrefix)
			}
		}
	}
	return nil
}

// applySecretMetadata stamps the spec's Secret annotations and labels onto secret,
// dropping keys the operator managed before that the spec no longer declares, and
// reports whether that changed anything. Data is never touched.
func applySecretMetadata(secret *corev1.Secret, virtualKey *litellmv1alpha1.LiteLLMVirtualKey) bool {
	annotations, managedAnnotations := mergeManaged(secret.Annotations, virtualKey.Spec.SecretAnnotations,
		previouslyManaged(secret.Annotations[managedAnnotationKeysAnnotation]))
	labels, managedLabels := mergeManaged(secret.Labels, virtualKey.Spec.SecretLabels,
		previouslyManaged(secret.Annotations[managedLabelKeysAnnotation]))
	recordManaged(annotations, managedAnnotationKeysAnnotation, managedAnnotations)
	recordManaged(annotations, managedLabelKeysAnnotation, managedLabels)
	// Leave an empty map absent rather than writing `annotations: {}` back.
	if len(annotations) == 0 {
		annotations = nil
	}
	if len(labels) == 0 {
		labels = nil
	}

	if maps.Equal(annotations, secret.Annotations) && maps.Equal(labels, secret.Labels) {
		return false
	}
	secret.Annotations, secret.Labels = annotations, labels
	return true
}

// mergeManaged applies desired onto a copy of current, removes previously managed
// keys desired no longer declares, and returns the result with the keys now
// managed. Keys the operator has never managed survive untouched.
func mergeManaged(current, desired map[string]string, previous []string) (map[string]string, []string) {
	merged := maps.Clone(current)
	if merged == nil {
		merged = make(map[string]string, len(desired))
	}
	for _, key := range previous {
		if _, ok := desired[key]; !ok {
			delete(merged, key)
		}
	}
	maps.Copy(merged, desired)
	return merged, slices.Sorted(maps.Keys(desired))
}

func previouslyManaged(record string) []string {
	if record == "" {
		return nil
	}
	return strings.Split(record, ",")
}

func recordManaged(annotations map[string]string, name string, keys []string) {
	if len(keys) == 0 {
		delete(annotations, name)
		return
	}
	annotations[name] = strings.Join(keys, ",")
}

func virtualKeyRequestsEqual(live litellmclient.VirtualKey, desired litellmclient.VirtualKeyRequest) bool {
	return live.KeyAlias == desired.KeyAlias &&
		slices.Equal(live.Models, desired.Models) &&
		maps.Equal(live.Aliases, desired.Aliases) &&
		live.UserID == desired.UserID && live.TeamID == desired.TeamID &&
		reflect.DeepEqual(live.MaxBudget, desired.MaxBudget) &&
		live.BudgetDuration == desired.BudgetDuration &&
		reflect.DeepEqual(live.MaxParallelRequests, desired.MaxParallelRequests) &&
		reflect.DeepEqual(live.TPMLimit, desired.TPMLimit) &&
		reflect.DeepEqual(live.RPMLimit, desired.RPMLimit) &&
		maps.Equal(live.Metadata, desired.Metadata)
}

func (r *LiteLLMVirtualKeyReconciler) reconcileDelete(ctx context.Context, virtualKey *litellmv1alpha1.LiteLLMVirtualKey) error {
	if !controllerutil.ContainsFinalizer(virtualKey, virtualKeyFinalizer) {
		return nil
	}
	secret, err := r.outputSecret(ctx, virtualKey)
	if err != nil {
		return fmt.Errorf("get output Secret for deletion: %w", err)
	}
	key, ok := secret.Data[virtualKey.SecretDataKey()]
	if !ok || len(key) == 0 {
		return fmt.Errorf("output Secret %s/%s has no key %q", virtualKey.Namespace, secret.Name, virtualKey.SecretDataKey())
	}
	admin, err := r.adminClient(ctx, virtualKey)
	if err != nil {
		return err
	}
	if err := admin.DeleteVirtualKey(ctx, string(key)); err != nil {
		return fmt.Errorf("delete LiteLLM key: %w", err)
	}
	controllerutil.RemoveFinalizer(virtualKey, virtualKeyFinalizer)
	if err := r.Update(ctx, virtualKey); err != nil {
		return fmt.Errorf("remove finalizer: %w", err)
	}
	return nil
}

func (r *LiteLLMVirtualKeyReconciler) outputSecret(ctx context.Context, virtualKey *litellmv1alpha1.LiteLLMVirtualKey) (*corev1.Secret, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: virtualKey.Namespace, Name: virtualKey.Spec.SecretName}, &secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func (r *LiteLLMVirtualKeyReconciler) adminClient(ctx context.Context, virtualKey *litellmv1alpha1.LiteLLMVirtualKey) (*litellmclient.Client, error) {
	var proxy litellmv1alpha1.LiteLLMProxy
	if err := r.Get(ctx, types.NamespacedName{Namespace: virtualKey.Namespace, Name: virtualKey.Spec.ProxyRef}, &proxy); err != nil {
		return nil, fmt.Errorf("get proxy: %w", err)
	}
	if proxy.Spec.APIAccess == nil {
		return nil, fmt.Errorf("proxy %s has no spec.apiAccess", proxy.Name)
	}
	masterKey, err := readSecretKey(ctx, r.Client, proxy.Namespace, proxy.Spec.APIAccess.MasterKeyRef)
	if err != nil {
		return nil, fmt.Errorf("read proxy master key: %w", err)
	}
	endpoint := proxy.Spec.APIAccess.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("http://%s.%s.svc:%d", proxy.Name, proxy.Namespace, servicePort(&proxy))
	}
	return litellmclient.New(endpoint, masterKey, nil), nil
}

func virtualKeyRequest(key *litellmv1alpha1.LiteLLMVirtualKey) (litellmclient.VirtualKeyRequest, error) {
	request := litellmclient.VirtualKeyRequest{
		KeyAlias:            key.Spec.KeyAlias,
		Models:              key.Spec.Models,
		Aliases:             key.Spec.Aliases,
		UserID:              key.Spec.UserID,
		TeamID:              key.Spec.TeamID,
		Duration:            key.Spec.Duration,
		BudgetDuration:      key.Spec.BudgetDuration,
		MaxParallelRequests: key.Spec.MaxParallelRequests,
		TPMLimit:            key.Spec.TPMLimit,
		RPMLimit:            key.Spec.RPMLimit,
		Metadata:            key.Spec.Metadata,
	}
	if key.Spec.MaxBudget == "" {
		return request, nil
	}
	maxBudget, err := strconv.ParseFloat(key.Spec.MaxBudget, 64)
	if err != nil {
		return litellmclient.VirtualKeyRequest{}, fmt.Errorf("parse maxBudget: %w", err)
	}
	request.MaxBudget = &maxBudget
	return request, nil
}

func (r *LiteLLMVirtualKeyReconciler) markReady(ctx context.Context, key *litellmv1alpha1.LiteLLMVirtualKey) error {
	meta.SetStatusCondition(&key.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             conditionReasonReconciled,
		Message:            "virtual key reconciled",
		ObservedGeneration: key.Generation,
	})
	return r.Status().Update(ctx, key)
}

func (r *LiteLLMVirtualKeyReconciler) markFailed(ctx context.Context, key *litellmv1alpha1.LiteLLMVirtualKey, reason, message string) error {
	meta.SetStatusCondition(&key.Status.Conditions, metav1.Condition{
		Type:               conditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: key.Generation,
	})
	if err := r.Status().Update(ctx, key); err != nil && !apierrors.IsConflict(err) {
		return err
	}
	return fmt.Errorf("%s: %s", reason, message)
}

// SetupWithManager wires the controller and its owned Secret watch.
func (r *LiteLLMVirtualKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&litellmv1alpha1.LiteLLMVirtualKey{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

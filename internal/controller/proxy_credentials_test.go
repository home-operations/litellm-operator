package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	testCredentialUnmanagedKey = "unrelated"
	testCredentialSourceName   = "source"
)

func credentialReconciler(t *testing.T, proxy *litellmv1alpha1.LiteLLMProxy, objects ...client.Object) *LiteLLMProxyReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, litellmv1alpha1.AddToScheme(scheme))
	require.NoError(t, gatewayv1.Install(scheme))
	return &LiteLLMProxyReconciler{Scheme: scheme, Client: fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(proxy).WithObjects(append(objects, proxy)...).Build()}
}

func TestProxyCredentialReferences(t *testing.T) {
	for _, namespace := range []string{testProxyNamespace, "apps"} {
		for _, mode := range []string{"file", "api"} {
			t.Run(namespace+"/"+mode, func(t *testing.T) {
				api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))
					if req.URL.Path == "/model/info" {
						_, _ = w.Write([]byte(`{"data":[]}`))
						return
					}
					assert.Equal(t, "/model/new", req.URL.Path)
					_, _ = w.Write([]byte(`{}`))
				}))
				defer api.Close()
				ref := &litellmv1alpha1.SecretKeyRef{Name: testCredentialSourceName, Key: "key"}
				baseRef := &litellmv1alpha1.SecretKeyRef{Name: ref.Name, Key: "base"}
				proxy := &litellmv1alpha1.LiteLLMProxy{
					ObjectMeta: metav1.ObjectMeta{Name: testProxyName, Namespace: testProxyNamespace, UID: "proxy-uid"},
					Spec: litellmv1alpha1.LiteLLMProxySpec{
						ApplyMode: mode, APIAccess: &litellmv1alpha1.APIAccessSpec{
							Endpoint: api.URL, MasterKeyRef: *ref,
						},
					},
				}
				resourceMeta := metav1.ObjectMeta{Name: "credential-resource", Namespace: namespace}
				model := &litellmv1alpha1.LiteLLMModel{ObjectMeta: resourceMeta,
					Spec: litellmv1alpha1.LiteLLMModelSpec{
						ProxyRef: proxy.Name, ProxyNamespace: proxy.Namespace, ModelName: "model",
						Params: litellmv1alpha1.LiteLLMParams{Model: "openai/model", APIKeyRef: ref, APIBaseRef: baseRef},
					}}
				guardrail := &litellmv1alpha1.LiteLLMGuardrail{ObjectMeta: resourceMeta,
					Spec: litellmv1alpha1.LiteLLMGuardrailSpec{
						ProxyRef: proxy.Name, ProxyNamespace: proxy.Namespace, GuardrailName: "guardrail", Guardrail: "presidio",
						APIKeyRef: ref, APIBaseRef: baseRef,
					}}
				server := &litellmv1alpha1.LiteLLMMCPServer{ObjectMeta: resourceMeta,
					Spec: litellmv1alpha1.LiteLLMMCPServerSpec{
						ProxyRef: proxy.Name, ProxyNamespace: proxy.Namespace, URL: "https://example.com/mcp", AuthTokenRef: ref,
					}}
				source := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: namespace},
					Data: map[string][]byte{ref.Key: []byte("test-key"), baseRef.Key: []byte("https://provider.example"), "unused": []byte("keep-local")}}
				objects := []client.Object{model, guardrail, server, source}
				if namespace != proxy.Namespace {
					objects = append(objects, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: proxy.Namespace},
						Data: map[string][]byte{ref.Key: []byte("test-key"), baseRef.Key: []byte("wrong-namespace")}})
				}
				r := credentialReconciler(t, proxy, objects...)
				req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(proxy)}
				_, err := r.Reconcile(t.Context(), req)
				require.NoError(t, err)
				var deploy appsv1.Deployment
				require.NoError(t, r.Get(t.Context(), req.NamespacedName, &deploy))
				var credentials corev1.Secret
				credentialsKey := types.NamespacedName{Name: proxy.Name + "-credentials", Namespace: proxy.Namespace}
				if namespace == proxy.Namespace {
					assert.True(t, apierrors.IsNotFound(r.Get(t.Context(), credentialsKey, &credentials)))
					assert.NotContains(t, deploy.Spec.Template.Annotations, credentialsHashAnnotation)
				} else {
					require.NoError(t, r.Get(t.Context(), credentialsKey, &credentials))
					assert.Len(t, credentials.Data, 5)
					assert.True(t, metav1.IsControlledBy(&credentials, proxy))
					assert.NotEmpty(t, deploy.Spec.Template.Annotations[credentialsHashAnnotation])
				}
				env := deploy.Spec.Template.Spec.Containers[0].Env
				require.Len(t, env, 5)
				want := map[string][]byte{
					model.APIKeyEnvVarName(): source.Data[ref.Key], model.APIBaseEnvVarName(): source.Data[baseRef.Key],
					guardrail.APIKeyEnvVarName(): source.Data[ref.Key], guardrail.APIBaseEnvVarName(): source.Data[baseRef.Key],
					server.AuthTokenEnvVarName(): source.Data[ref.Key],
				}
				for _, variable := range env {
					secretRef := variable.ValueFrom.SecretKeyRef
					var secret corev1.Secret
					require.NoError(t, r.Get(t.Context(), types.NamespacedName{Name: secretRef.Name, Namespace: proxy.Namespace}, &secret))
					assert.Equal(t, want[variable.Name], secret.Data[secretRef.Key])
				}
				var cm corev1.ConfigMap
				require.NoError(t, r.Get(t.Context(), types.NamespacedName{Name: configMapName(proxy), Namespace: proxy.Namespace}, &cm))
				assert.NotContains(t, cm.Data[configFileName], "test-key")
				assert.NotContains(t, cm.Data[configFileName], "https://provider.example")
				assert.Equal(t, []reconcile.Request{req}, r.proxiesForSecret(t.Context(), source))
				assert.Empty(t, r.proxiesForSecret(t.Context(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: testCredentialUnmanagedKey, Namespace: namespace}}))
				_, err = r.Reconcile(t.Context(), req)
				require.NoError(t, err)
				var unchanged appsv1.Deployment
				require.NoError(t, r.Get(t.Context(), req.NamespacedName, &unchanged))
				assert.Equal(t, deploy.ResourceVersion, unchanged.ResourceVersion)
				if namespace != proxy.Namespace {
					var unchangedSecret corev1.Secret
					require.NoError(t, r.Get(t.Context(), credentialsKey, &unchangedSecret))
					assert.Equal(t, credentials.ResourceVersion, unchangedSecret.ResourceVersion)
				}
				require.NoError(t, r.Delete(t.Context(), model))
				_, err = r.Reconcile(t.Context(), req)
				require.NoError(t, err)
				if namespace != proxy.Namespace {
					require.NoError(t, r.Get(t.Context(), credentialsKey, &credentials))
					assert.Len(t, credentials.Data, 3)
					assert.NotContains(t, credentials.Data, model.APIKeyEnvVarName())
					assert.NotContains(t, credentials.Data, model.APIBaseEnvVarName())
				}
				require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(guardrail), guardrail))
				guardrail.Spec.APIKeyRef, guardrail.Spec.APIBaseRef = nil, nil
				require.NoError(t, r.Update(t.Context(), guardrail))
				_, err = r.Reconcile(t.Context(), req)
				require.NoError(t, err)
				if namespace != proxy.Namespace {
					require.NoError(t, r.Get(t.Context(), credentialsKey, &credentials))
					assert.Equal(t, map[string][]byte{server.AuthTokenEnvVarName(): source.Data[ref.Key]}, credentials.Data)
				}
				require.NoError(t, r.Delete(t.Context(), server))
				_, err = r.Reconcile(t.Context(), req)
				require.NoError(t, err)
				assert.True(t, apierrors.IsNotFound(r.Get(t.Context(), credentialsKey, &credentials)))
				require.NoError(t, r.Get(t.Context(), req.NamespacedName, &deploy))
				assert.Empty(t, deploy.Spec.Template.Spec.Containers[0].Env)
				assert.NotContains(t, deploy.Spec.Template.Annotations, credentialsHashAnnotation)
				var unchangedSource corev1.Secret
				require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(source), &unchangedSource))
				assert.Equal(t, source.Data, unchangedSource.Data)
			})
		}
	}
}

func TestProxyCredentialFailures(t *testing.T) {
	for _, tt := range []struct {
		name       string
		sourceData map[string][]byte
		conflict   bool
		wantError  string
	}{
		{name: "missing secret", wantError: `secrets "source" not found`},
		{name: "missing key", sourceData: map[string][]byte{}, wantError: `secret apps/source has no key "credential"`},
		{name: "unowned copy", sourceData: map[string][]byte{"credential": []byte("test-token")}, conflict: true, wantError: "is not owned by this proxy"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &litellmv1alpha1.LiteLLMProxy{ObjectMeta: metav1.ObjectMeta{Name: testProxyName, Namespace: testProxyNamespace, UID: "proxy-uid"}}
			server := &litellmv1alpha1.LiteLLMMCPServer{ObjectMeta: metav1.ObjectMeta{Name: "credential-resource", Namespace: "apps"},
				Spec: litellmv1alpha1.LiteLLMMCPServerSpec{
					ProxyRef: proxy.Name, ProxyNamespace: proxy.Namespace, URL: "https://example.com/mcp",
					AuthTokenRef: &litellmv1alpha1.SecretKeyRef{Name: testCredentialSourceName, Key: "credential"},
				}}
			objects := []client.Object{server}
			if tt.sourceData != nil {
				objects = append(objects, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: server.Spec.AuthTokenRef.Name, Namespace: server.Namespace}, Data: tt.sourceData})
			}
			key := types.NamespacedName{Name: proxy.Name + "-credentials", Namespace: proxy.Namespace}
			if tt.conflict {
				objects = append(objects, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
					Data: map[string][]byte{testCredentialUnmanagedKey: []byte("keep")}})
			}
			r := credentialReconciler(t, proxy, objects...)
			req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(proxy)}
			_, err := r.Reconcile(t.Context(), req)
			require.ErrorContains(t, err, tt.wantError)
			require.NoError(t, r.Get(t.Context(), req.NamespacedName, proxy))
			condition := meta.FindStatusCondition(proxy.Status.Conditions, conditionTypeReady)
			require.NotNil(t, condition)
			assert.Equal(t, metav1.ConditionFalse, condition.Status)
			assert.Equal(t, "SecretResolutionFailed", condition.Reason)
			assert.NotContains(t, condition.Message, "test-token")
			var deploy appsv1.Deployment
			assert.True(t, apierrors.IsNotFound(r.Get(t.Context(), req.NamespacedName, &deploy)))
			require.NoError(t, r.Delete(t.Context(), server))
			_, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			var credentials corev1.Secret
			if tt.conflict {
				require.NoError(t, r.Get(t.Context(), key, &credentials))
				assert.Equal(t, map[string][]byte{testCredentialUnmanagedKey: []byte("keep")}, credentials.Data)
				assert.Empty(t, credentials.OwnerReferences)
			} else {
				assert.True(t, apierrors.IsNotFound(r.Get(t.Context(), key, &credentials)))
			}
		})
	}
}

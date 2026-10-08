package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	testMCPAPIPath       = "/v1/mcp/server"
	testMCPInfoField     = "mcp_info"
	testMCPServerIDField = "server_id"
	testMCPTokenKey      = "installation_token"
)

func TestAPIMCPCredentialRotation(t *testing.T) {
	for _, namespace := range []string{testProxyNamespace, "mcp-apps"} {
		t.Run(namespace, func(t *testing.T) {
			var registered []map[string]any
			var tokens []any
			failUpdate := false
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				assert.Equal(t, "Bearer master-key", req.Header.Get("Authorization"))
				switch {
				case req.URL.Path == "/model/info":
					_, _ = w.Write([]byte(`{"data":[]}`))
				case req.URL.Path == testMCPAPIPath && req.Method == http.MethodGet:
					if registered == nil {
						registered = []map[string]any{}
					}
					assert.NoError(t, json.NewEncoder(w).Encode(registered))
				case req.URL.Path == testMCPAPIPath:
					var body map[string]any
					assert.NoError(t, json.NewDecoder(req.Body).Decode(&body))
					credentials, _ := body["credentials"].(map[string]any)
					tokens = append(tokens, credentials["auth_value"])
					if len(tokens) == 1 {
						assert.Equal(t, http.MethodPost, req.Method)
					} else {
						assert.Equal(t, http.MethodPut, req.Method)
					}
					body["credentials"] = nil
					registered = []map[string]any{body}
					if failUpdate {
						http.Error(w, fmt.Sprint(credentials["auth_value"]), http.StatusServiceUnavailable)
						return
					}
					_, _ = w.Write([]byte(`{}`))
				case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/v1/mcp/server/"):
					registered = nil
					_, _ = w.Write([]byte(`{}`))
				default:
					t.Errorf("unexpected API request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer api.Close()
			proxy := &litellmv1alpha1.LiteLLMProxy{
				ObjectMeta: metav1.ObjectMeta{Name: testProxyName, Namespace: testProxyNamespace, UID: "mcp-proxy-uid"},
				Spec: litellmv1alpha1.LiteLLMProxySpec{ApplyMode: applyModeAPI, APIAccess: &litellmv1alpha1.APIAccessSpec{
					Endpoint: api.URL, MasterKeyRef: litellmv1alpha1.SecretKeyRef{Name: testMasterSecretName, Key: testMasterSecretKey},
				}},
			}
			source := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "mcp-token", Namespace: namespace},
				Data: map[string][]byte{testMCPTokenKey: []byte("installation-token-1")}}
			server := &litellmv1alpha1.LiteLLMMCPServer{
				ObjectMeta: metav1.ObjectMeta{Name: "github-api", Namespace: namespace, UID: "server-uid"},
				Spec: litellmv1alpha1.LiteLLMMCPServerSpec{ProxyRef: proxy.Name, ProxyNamespace: proxy.Namespace,
					URL: "https://github.example/mcp", Transport: "http", AuthType: "bearer_token",
					AuthTokenRef: &litellmv1alpha1.SecretKeyRef{Name: source.Name, Key: testMCPTokenKey},
				},
			}
			master := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: proxy.Namespace},
				Data: map[string][]byte{testMasterSecretKey: []byte("master-key")}}
			r := credentialReconciler(t, proxy, source, server, master)
			req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(proxy)}
			_, err := r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, []any{"installation-token-1"}, tokens)
			var before appsv1.Deployment
			require.NoError(t, r.Get(t.Context(), req.NamespacedName, &before))
			assert.Empty(t, before.Spec.Template.Spec.Containers[0].Env)
			assert.NotContains(t, before.Spec.Template.Annotations, credentialsHashAnnotation)
			var cm corev1.ConfigMap
			require.NoError(t, r.Get(t.Context(), client.ObjectKey{Namespace: proxy.Namespace, Name: configMapName(proxy)}, &cm))
			assert.NotContains(t, cm.Data[configFileName], "mcp_servers")
			assert.NotContains(t, cm.Data[configFileName], "installation-token")
			var copied corev1.Secret
			assert.True(t, apierrors.IsNotFound(r.Get(t.Context(), client.ObjectKey{
				Namespace: proxy.Namespace, Name: proxy.Name + "-credentials",
			}, &copied)))

			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(source), source))
			source.Data["unused"] = []byte("unrelated-secret-change")
			require.NoError(t, r.Update(t.Context(), source))
			_, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Len(t, tokens, 1)
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(source), source))
			source.Data[testMCPTokenKey] = []byte("installation-token-2")
			require.NoError(t, r.Update(t.Context(), source))
			assert.Equal(t, []reconcile.Request{req}, r.proxiesForSecret(t.Context(), source))
			failUpdate = true
			_, err = r.Reconcile(t.Context(), req)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "installation-token-2")
			require.NoError(t, r.Get(t.Context(), req.NamespacedName, proxy))
			condition := meta.FindStatusCondition(proxy.Status.Conditions, conditionTypeReady)
			require.NotNil(t, condition)
			assert.Equal(t, metav1.ConditionFalse, condition.Status)
			assert.NotContains(t, condition.Message, "installation-token-2")
			failUpdate = false
			_, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Equal(t, []any{"installation-token-1", "installation-token-2", "installation-token-2"}, tokens)
			var after appsv1.Deployment
			require.NoError(t, r.Get(t.Context(), req.NamespacedName, &after))
			assert.Equal(t, before.Spec.Template, after.Spec.Template)
			assert.Equal(t, before.ResourceVersion, after.ResourceVersion)

			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(source), source))
			delete(source.Data, testMCPTokenKey)
			require.NoError(t, r.Update(t.Context(), source))
			_, err = r.Reconcile(t.Context(), req)
			require.ErrorContains(t, err, "has no key")
			assert.Len(t, tokens, 3)
			source.Data[testMCPTokenKey] = []byte("installation-token-2")
			require.NoError(t, r.Update(t.Context(), source))

			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(server), server))
			server.Spec.AuthTokenRef = nil
			require.NoError(t, r.Update(t.Context(), server))
			_, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Nil(t, tokens[len(tokens)-1])
			require.NoError(t, r.Get(t.Context(), req.NamespacedName, proxy))
			proxy.Spec.ApplyMode = "file"
			require.NoError(t, r.Update(t.Context(), proxy))
			_, err = r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			assert.Empty(t, registered)
			require.NoError(t, r.Get(t.Context(), client.ObjectKey{Namespace: proxy.Namespace, Name: configMapName(proxy)}, &cm))
			assert.Contains(t, cm.Data[configFileName], "mcp_servers")
		})
	}
}

func TestAPIMCPDeletionOwnership(t *testing.T) {
	proxy := &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: testProxyName, Namespace: testProxyNamespace, UID: "deletion-proxy"},
		Spec: litellmv1alpha1.LiteLLMProxySpec{APIAccess: &litellmv1alpha1.APIAccessSpec{
			MasterKeyRef: litellmv1alpha1.SecretKeyRef{Name: testMasterSecretName, Key: testMasterSecretKey},
		}},
		Status: litellmv1alpha1.LiteLLMProxyStatus{MCPConfigHash: "previous-sync"},
	}
	owner := fmt.Sprintf("%s/%s/%s", proxy.Namespace, proxy.Name, proxy.UID)
	existing := []map[string]any{
		{testMCPServerIDField: "owned", testMCPInfoField: map[string]any{managedByKey: managedByValue, mcpProxyOwnerKey: owner}},
		{testMCPServerIDField: "foreign", testMCPInfoField: map[string]any{managedByKey: managedByValue, mcpProxyOwnerKey: "another-proxy"}},
		{testMCPServerIDField: "manual", testMCPInfoField: map[string]any{}},
	}
	var deleted []string
	failDelete := true
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			assert.NoError(t, json.NewEncoder(w).Encode(existing))
			return
		}
		assert.Equal(t, http.MethodDelete, req.Method)
		assert.Equal(t, testMCPAPIPath+"/owned", req.URL.Path)
		if failDelete {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		deleted = append(deleted, "owned")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer api.Close()
	proxy.Spec.APIAccess.Endpoint = api.URL
	master := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: proxy.Namespace},
		Data: map[string][]byte{testMasterSecretKey: []byte("deletion-master")}}
	r := credentialReconciler(t, proxy, master)
	require.Error(t, r.syncMCPServersViaAPI(t.Context(), proxy, nil))
	assert.Equal(t, "previous-sync", proxy.Status.MCPConfigHash)
	failDelete = false
	require.NoError(t, r.syncMCPServersViaAPI(t.Context(), proxy, nil))
	assert.Equal(t, []string{"owned"}, deleted)
}

func TestAPIMCPDatabaseLoading(t *testing.T) {
	for _, tc := range []struct {
		name      string
		settings  string
		wantError bool
	}{
		{name: "implicit loading"},
		{name: "mcp enabled", settings: `{"supported_db_objects":["mcp"]}`},
		{name: "mcp excluded", settings: `{"supported_db_objects":["guardrails"]}`, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy := &litellmv1alpha1.LiteLLMProxy{Spec: litellmv1alpha1.LiteLLMProxySpec{
				ApplyMode: applyModeAPI, GeneralSettings: raw(tc.settings),
			}}
			_, err := renderConfig(proxy, nil, nil, []litellmv1alpha1.LiteLLMMCPServer{{
				ObjectMeta: metav1.ObjectMeta{Name: "database-mcp"},
			}})
			if tc.wantError {
				require.ErrorContains(t, err, "supported_db_objects")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

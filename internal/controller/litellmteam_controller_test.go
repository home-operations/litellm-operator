package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const (
	testTeamName    = "apps"
	testTeamIDField = "team_id"
)

func TestLiteLLMTeamReconciler_MCPServers(t *testing.T) {
	tests := []struct {
		name           string
		servers        []string
		want           []any
		proxyNamespace string
	}{
		{name: "replace servers", servers: []string{"ha-mcp", "context7"}, want: []any{"ha-mcp", "context7"}},
		{name: "deny access", servers: []string{"no-mcp-servers"}, want: []any{"no-mcp-servers"}},
		{name: "empty list", servers: []string{}, want: []any{}},
		{name: "omitted field", want: []any{}},
		{name: "cross namespace", servers: []string{"remote-mcp"}, want: []any{"remote-mcp"}, proxyNamespace: testProxyNamespace},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var creates, updates []map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				assert.Equal(t, "Bearer master", req.Header.Get("Authorization"))
				if req.URL.Path == "/team/info" {
					_ = json.NewEncoder(w).Encode(map[string]any{"team_info": map[string]any{
						testTeamIDField: testTeamName, "members_with_roles": []any{},
					}})
					return
				}
				var body map[string]any
				require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
				switch req.URL.Path {
				case "/team/new":
					creates = append(creates, body)
					_ = json.NewEncoder(w).Encode(map[string]string{testTeamIDField: testTeamName})
				case "/team/update":
					updates = append(updates, body)
				default:
					t.Errorf("unexpected request: %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			team := &litellmv1alpha1.LiteLLMTeam{
				ObjectMeta: metav1.ObjectMeta{Name: testTeamName, Namespace: metav1.NamespaceDefault, Generation: 1},
				Spec:       litellmv1alpha1.LiteLLMTeamSpec{ProxyRef: testProxyName, ProxyNamespace: tt.proxyNamespace, MCPServers: []string{testOldKeyValue}},
			}
			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			require.NoError(t, litellmv1alpha1.AddToScheme(scheme))
			proxy := testVirtualKeyProxy(srv.URL)
			if tt.proxyNamespace != "" {
				proxy.Namespace = tt.proxyNamespace
			}
			master := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: proxy.Namespace},
				Data:       map[string][]byte{testMasterSecretKey: []byte("master")},
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(team).
				WithObjects(team, proxy, master).Build()
			r := &LiteLLMTeamReconciler{Client: c, Scheme: scheme}
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(team)}
			_, err := r.Reconcile(t.Context(), req)
			require.NoError(t, err)
			require.Len(t, creates, 1)
			assert.Equal(t, map[string]any{"mcp_servers": []any{testOldKeyValue}}, creates[0]["object_permission"])

			require.NoError(t, c.Get(t.Context(), req.NamespacedName, team))
			team.Spec.MCPServers = tt.servers
			team.Generation++
			require.NoError(t, c.Update(t.Context(), team))
			for range 2 {
				_, err := r.Reconcile(t.Context(), req)
				require.NoError(t, err)
			}
			require.Len(t, updates, 1)
			assert.Equal(t, map[string]any{"mcp_servers": tt.want}, updates[0]["object_permission"])
			require.NoError(t, c.Get(t.Context(), req.NamespacedName, team))
			assert.Equal(t, testTeamName, team.Status.TeamID)
			ready := meta.FindStatusCondition(team.Status.Conditions, conditionTypeReady)
			require.NotNil(t, ready)
			assert.Equal(t, metav1.ConditionTrue, ready.Status)
			assert.Equal(t, team.Generation, ready.ObservedGeneration)
		})
	}
}

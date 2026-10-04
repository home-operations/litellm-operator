package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
	"github.com/home-operations/litellm-operator/internal/litellmclient"
)

const (
	testKeyInfoField     = "info"
	testKeyModelsField   = "models"
	testOldKeyValue      = "old"
	testKeyUpdatePath    = "/key/update"
	testKeyDuration      = "30d"
	testProxyName        = "proxy"
	testMasterSecretName = "master"
	testMasterSecretKey  = "key"
	testKeyInfoPath      = "/key/info"
	testKeyAlias         = "application"
	testKeyModel         = "openai/gpt-5"
	testKeyTeam          = "team-a"
	testKeyToolset       = "toolset-a"

	testReflectorAnnotation = "reflector.v1.k8s.emberstack.com/reflection-allowed"
	testReflectorAllowed    = "true"
	// Metadata on the output Secret that the operator never manages.
	testUnmanagedKey   = "other"
	testUnmanagedValue = "keep"
	// A value some other writer put on a key the spec also names.
	testForeignValue = "theirs"

	testGeneratedKey    = "sk-generated"
	testAnnotationsSpec = "spec.secretAnnotations"
	testLabelsSpec      = "spec.secretLabels"
)

// writeTestKeyInfo answers /key/info with the key testVirtualKey asks for, so the
// reconciler sees the live key as already matching the spec.
func writeTestKeyInfo(w http.ResponseWriter, toolsets ...string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		testMasterSecretKey: testGeneratedKey,
		testKeyInfoField: map[string]any{
			"key_alias":         testKeyAlias,
			testKeyModelsField:  []string{testKeyModel},
			"team_id":           testKeyTeam,
			"object_permission": litellmclient.ObjectPermission{MCPToolsets: toolsets},
		},
	})
}

func TestLiteLLMVirtualKeyReconciler_ReconcileCreatesSecretOnce(t *testing.T) {
	var requests []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer master", r.Header.Get("Authorization"))
		if r.URL.Path == testKeyInfoPath {
			writeTestKeyInfo(w, testKeyToolset)
			return
		}
		assert.Equal(t, "/key/generate", r.URL.Path)
		var request map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		requests = append(requests, request)
		_ = json.NewEncoder(w).Encode(map[string]string{testMasterSecretKey: "sk-generated"})
	}))
	defer srv.Close()

	key := testVirtualKey()
	key.Spec.MCPToolsets = []string{testKeyToolset}
	key.Spec.SecretAnnotations = map[string]string{testReflectorAnnotation: testReflectorAllowed}
	key.Spec.SecretLabels = map[string]string{"app.kubernetes.io/part-of": testKeyAlias}
	proxy := testVirtualKeyProxy(srv.URL)
	masterKey := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace},
		Data:       map[string][]byte{testMasterSecretKey: []byte("master")},
	}
	r := testVirtualKeyReconciler(t, key, proxy, masterKey)
	request := types.NamespacedName{Namespace: key.Namespace, Name: key.Name}

	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: request})
	require.NoError(t, err)
	_, err = r.Reconcile(t.Context(), ctrl.Request{NamespacedName: request})
	require.NoError(t, err)

	var secret corev1.Secret
	require.NoError(t, r.Get(t.Context(), types.NamespacedName{Namespace: key.Namespace, Name: key.Spec.SecretName}, &secret))
	assert.Equal(t, []byte("sk-generated"), secret.Data["token"])
	assert.True(t, metav1.IsControlledBy(&secret, key))
	assert.Equal(t, testReflectorAllowed, secret.Annotations[testReflectorAnnotation])
	assert.Equal(t, testReflectorAnnotation, secret.Annotations[managedAnnotationKeysAnnotation])
	assert.Equal(t, testKeyAlias, secret.Labels["app.kubernetes.io/part-of"])
	assert.Equal(t, "app.kubernetes.io/part-of", secret.Annotations[managedLabelKeysAnnotation])
	require.Len(t, requests, 1)
	assert.Equal(t, testKeyAlias, requests[0]["key_alias"])
	assert.Equal(t, []any{testKeyModel}, requests[0][testKeyModelsField])
	assert.Equal(t, testKeyTeam, requests[0]["team_id"])
	assert.Equal(t, map[string]any{"mcp_toolsets": []any{testKeyToolset}}, requests[0]["object_permission"])
}

func TestLiteLLMVirtualKeyReconciler_CrossNamespace(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer master", r.Header.Get("Authorization"))
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/key/generate":
			_ = json.NewEncoder(w).Encode(map[string]string{testMasterSecretKey: testGeneratedKey})
		case testKeyInfoPath:
			writeTestKeyInfo(w)
		case "/key/delete":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	key := testVirtualKey()
	key.Spec.ProxyNamespace = testProxyNamespace
	proxy := testVirtualKeyProxy(srv.URL)
	proxy.Namespace = testProxyNamespace
	master := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: proxy.Namespace},
		Data:       map[string][]byte{testMasterSecretKey: []byte("master")},
	}
	localProxy := &litellmv1alpha1.LiteLLMProxy{ObjectMeta: metav1.ObjectMeta{Name: proxy.Name, Namespace: key.Namespace}}
	r := testVirtualKeyReconciler(t, key, proxy, master, localProxy)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name}}
	for range 2 {
		_, err := r.Reconcile(t.Context(), req)
		require.NoError(t, err)
	}
	var secret corev1.Secret
	secretKey := types.NamespacedName{Namespace: key.Namespace, Name: key.Spec.SecretName}
	require.NoError(t, r.Get(t.Context(), secretKey, &secret))
	assert.Equal(t, []byte(testGeneratedKey), secret.Data[key.SecretDataKey()])
	assert.True(t, metav1.IsControlledBy(&secret, key))
	secretKey.Namespace = proxy.Namespace
	assert.True(t, apierrors.IsNotFound(r.Get(t.Context(), secretKey, &corev1.Secret{})))

	require.NoError(t, r.Get(t.Context(), req.NamespacedName, key))
	require.NoError(t, r.Delete(t.Context(), key))
	_, err := r.Reconcile(t.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, []string{"/key/generate", testKeyInfoPath, "/key/delete"}, paths)
}

func TestLiteLLMVirtualKeyReconciler_ReconcileUpdatesChangedSpec(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case testKeyInfoPath:
			_ = json.NewEncoder(w).Encode(map[string]any{
				testMasterSecretKey: testGeneratedKey,
				testKeyInfoField:    map[string]any{"key_alias": testKeyAlias, testKeyModelsField: []string{testOldKeyValue}, "team_id": testKeyTeam},
			})
		case testKeyUpdatePath:
			var request map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			assert.Equal(t, []any{testKeyModel}, request[testKeyModelsField])
		}
	}))
	defer srv.Close()

	key := testVirtualKey()
	proxy := testVirtualKeyProxy(srv.URL)
	masterKey := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace}, Data: map[string][]byte{testMasterSecretKey: []byte("master")}}
	output := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: key.Spec.SecretName, Namespace: key.Namespace}, Data: map[string][]byte{key.SecretDataKey(): []byte("sk-generated")}}
	r := testVirtualKeyReconciler(t, key, proxy, masterKey, output)
	require.NoError(t, ctrl.SetControllerReference(key, output, r.Scheme))
	require.NoError(t, r.Update(t.Context(), output))

	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name}})
	require.NoError(t, err)
	assert.Equal(t, []string{testKeyInfoPath, testKeyUpdatePath}, paths)
}

func TestLiteLLMVirtualKeyReconciler_ReconcileKeySettings(t *testing.T) {
	expires := "2026-10-30T12:00:00"
	tests := []struct {
		name         string
		live         litellmclient.VirtualKey
		desired      litellmv1alpha1.LiteLLMVirtualKeySpec
		lastDuration string
		wantUpdates  int
		wantDuration string
	}{
		{
			name: "nil and empty collections match",
			live: litellmclient.VirtualKey{
				Models: []string{}, Aliases: map[string]string{}, Metadata: map[string]string{},
				ObjectPermission: &litellmclient.ObjectPermission{MCPToolsets: []string{}},
			},
		},
		{
			name:    "matching toolsets",
			live:    litellmclient.VirtualKey{ObjectPermission: &litellmclient.ObjectPermission{MCPToolsets: []string{testKeyToolset}}},
			desired: litellmv1alpha1.LiteLLMVirtualKeySpec{MCPToolsets: []string{testKeyToolset}},
		},
		{
			name:        "repair missing toolsets",
			desired:     litellmv1alpha1.LiteLLMVirtualKeySpec{MCPToolsets: []string{testKeyToolset}},
			wantUpdates: 1,
		},
		{
			name:        "change toolsets",
			live:        litellmclient.VirtualKey{ObjectPermission: &litellmclient.ObjectPermission{MCPToolsets: []string{testOldKeyValue}}},
			desired:     litellmv1alpha1.LiteLLMVirtualKeySpec{MCPToolsets: []string{testKeyToolset}},
			wantUpdates: 1,
		},
		{
			name:        "clear toolsets with empty list",
			live:        litellmclient.VirtualKey{ObjectPermission: &litellmclient.ObjectPermission{MCPToolsets: []string{testKeyToolset}}},
			desired:     litellmv1alpha1.LiteLLMVirtualKeySpec{MCPToolsets: []string{}},
			wantUpdates: 1,
		},
		{
			name:        "clear toolsets with omitted field",
			live:        litellmclient.VirtualKey{ObjectPermission: &litellmclient.ObjectPermission{MCPToolsets: []string{testKeyToolset}}},
			wantUpdates: 1,
		},
		{
			name:        "models change in place",
			live:        litellmclient.VirtualKey{Models: []string{testOldKeyValue}},
			desired:     litellmv1alpha1.LiteLLMVirtualKeySpec{Models: []string{"new"}},
			wantUpdates: 1,
		},
		{
			name: "clear settings",
			live: litellmclient.VirtualKey{
				KeyAlias: testOldKeyValue, Models: []string{testOldKeyValue}, Aliases: map[string]string{"alias": testOldKeyValue},
				UserID: "old-user", TeamID: "old-team", MaxBudget: new(12.5), BudgetDuration: "1d",
				MaxParallelRequests: new(int64(3)), TPMLimit: new(int64(100)), RPMLimit: new(int64(10)),
				Metadata: map[string]string{"app": testOldKeyValue},
			},
			wantUpdates: 1,
		},
		{
			name:         "unchanged duration does not renew expiry",
			live:         litellmclient.VirtualKey{Expires: &expires},
			desired:      litellmv1alpha1.LiteLLMVirtualKeySpec{Duration: testKeyDuration},
			lastDuration: testKeyDuration,
		},
		{
			name:         "changing models preserves expiry",
			live:         litellmclient.VirtualKey{Models: []string{testOldKeyValue}, Expires: &expires},
			desired:      litellmv1alpha1.LiteLLMVirtualKeySpec{Models: []string{"new"}, Duration: testKeyDuration},
			lastDuration: testKeyDuration, wantUpdates: 1,
		},
		{
			name:         "change duration once",
			live:         litellmclient.VirtualKey{Expires: &expires},
			desired:      litellmv1alpha1.LiteLLMVirtualKeySpec{Duration: "2d"},
			lastDuration: testKeyDuration, wantUpdates: 1, wantDuration: "2d",
		},
		{
			name:         "clear duration",
			live:         litellmclient.VirtualKey{Expires: &expires},
			lastDuration: testKeyDuration, wantUpdates: 1, wantDuration: "null",
		},
		{
			name:        "adopt existing key with duration",
			live:        litellmclient.VirtualKey{Expires: &expires},
			desired:     litellmv1alpha1.LiteLLMVirtualKeySpec{Duration: testKeyDuration},
			wantUpdates: 1, wantDuration: testKeyDuration,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			live := tt.live
			var updates []map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch req.URL.Path {
				case testKeyInfoPath:
					assert.Equal(t, testGeneratedKey, req.URL.Query().Get(testMasterSecretKey))
					_ = json.NewEncoder(w).Encode(map[string]any{testMasterSecretKey: testGeneratedKey, testKeyInfoField: live})
				case testKeyUpdatePath:
					var body map[string]any
					require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
					assert.Equal(t, testGeneratedKey, body[testMasterSecretKey])
					updates = append(updates, body)
					encoded, err := json.Marshal(body)
					require.NoError(t, err)
					var next litellmclient.VirtualKey
					require.NoError(t, json.Unmarshal(encoded, &next))
					next.Expires = live.Expires
					if duration, ok := body["duration"]; ok {
						next.Expires = nil
						if duration != nil {
							next.Expires = new("2026-10-31T12:00:00")
						}
					}
					next.Duration = ""
					live = next
				default:
					t.Errorf("unexpected request: %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			key := testVirtualKey()
			key.Spec = tt.desired
			key.Spec.ProxyRef = testProxyName
			key.Spec.SecretName = "application-key"
			key.Generation = 2
			output := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: key.Spec.SecretName, Namespace: key.Namespace,
					Annotations: map[string]string{managedDurationAnnotation: tt.lastDuration},
				},
				Data: map[string][]byte{key.SecretDataKey(): []byte(testGeneratedKey)},
			}
			masterKey := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace},
				Data:       map[string][]byte{testMasterSecretKey: []byte("master")},
			}
			r := testVirtualKeyReconciler(t, key, testVirtualKeyProxy(srv.URL), masterKey, output)
			require.NoError(t, ctrl.SetControllerReference(key, output, r.Scheme))
			require.NoError(t, r.Update(t.Context(), output))
			request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name}}
			for range 2 {
				_, err := r.Reconcile(t.Context(), request)
				require.NoError(t, err)
			}
			require.Len(t, updates, tt.wantUpdates)
			if tt.wantUpdates != 0 {
				switch tt.wantDuration {
				case "":
					assert.NotContains(t, updates[0], "duration")
				case "null":
					assert.Contains(t, updates[0], "duration")
					assert.Nil(t, updates[0]["duration"])
				default:
					assert.Equal(t, tt.wantDuration, updates[0]["duration"])
				}
			}
			assert.Equal(t, key.Spec.KeyAlias, live.KeyAlias)
			assert.ElementsMatch(t, key.Spec.Models, live.Models)
			if live.ObjectPermission != nil {
				assert.ElementsMatch(t, key.Spec.MCPToolsets, live.ObjectPermission.MCPToolsets)
			}
			assert.Equal(t, key.Spec.UserID, live.UserID)
			assert.Equal(t, key.Spec.TeamID, live.TeamID)
			assert.Equal(t, key.Spec.BudgetDuration, live.BudgetDuration)
			assert.Equal(t, key.Spec.MaxParallelRequests, live.MaxParallelRequests)
			assert.Equal(t, key.Spec.TPMLimit, live.TPMLimit)
			assert.Equal(t, key.Spec.RPMLimit, live.RPMLimit)
			assert.Nil(t, live.MaxBudget)
			assert.Empty(t, live.Aliases)
			assert.Empty(t, live.Metadata)
			require.NoError(t, r.Get(t.Context(), request.NamespacedName, key))
			ready := meta.FindStatusCondition(key.Status.Conditions, conditionTypeReady)
			require.NotNil(t, ready)
			assert.Equal(t, metav1.ConditionTrue, ready.Status)
			assert.Equal(t, key.Generation, ready.ObservedGeneration)
			require.NoError(t, r.Get(t.Context(), types.NamespacedName{Namespace: key.Namespace, Name: output.Name}, output))
			assert.Equal(t, []byte(testGeneratedKey), output.Data[key.SecretDataKey()])
			assert.Equal(t, key.Spec.Duration, output.Annotations[managedDurationAnnotation])
			if tt.desired.Duration != "" && tt.wantDuration == "" {
				assert.Equal(t, tt.live.Expires, live.Expires)
			}
		})
	}
}

func TestLiteLLMVirtualKeyReconciler_ReconcileAPIFailure(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		reason string
	}{
		{name: "read fails", path: testKeyInfoPath, reason: "GetFailed"},
		{name: "update fails", path: testKeyUpdatePath, reason: "UpdateFailed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == tt.path {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				assert.Equal(t, testKeyInfoPath, req.URL.Path)
				_ = json.NewEncoder(w).Encode(map[string]any{testKeyInfoField: map[string]any{testKeyModelsField: []string{testOldKeyValue}}})
			}))
			defer srv.Close()
			key := testVirtualKey()
			key.Generation = 2
			key.Spec.Duration = testKeyDuration
			key.Status.Conditions = []metav1.Condition{{
				Type: conditionTypeReady, Status: metav1.ConditionTrue, Reason: conditionReasonReconciled,
				ObservedGeneration: 1, LastTransitionTime: metav1.Now(),
			}}
			output := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: key.Spec.SecretName, Namespace: key.Namespace},
				Data:       map[string][]byte{key.SecretDataKey(): []byte(testGeneratedKey)},
			}
			masterKey := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace},
				Data:       map[string][]byte{testMasterSecretKey: []byte("master")},
			}
			r := testVirtualKeyReconciler(t, key, testVirtualKeyProxy(srv.URL), masterKey, output)
			require.NoError(t, ctrl.SetControllerReference(key, output, r.Scheme))
			require.NoError(t, r.Update(t.Context(), output))
			request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name}}
			_, err := r.Reconcile(t.Context(), request)
			require.ErrorContains(t, err, tt.reason)
			require.NoError(t, r.Get(t.Context(), request.NamespacedName, key))
			ready := meta.FindStatusCondition(key.Status.Conditions, conditionTypeReady)
			require.NotNil(t, ready)
			assert.Equal(t, metav1.ConditionFalse, ready.Status)
			assert.Equal(t, tt.reason, ready.Reason)
			assert.Equal(t, key.Generation, ready.ObservedGeneration)
			require.NoError(t, r.Get(t.Context(), types.NamespacedName{Namespace: key.Namespace, Name: output.Name}, output))
			assert.Equal(t, []byte(testGeneratedKey), output.Data[key.SecretDataKey()])
			assert.NotContains(t, output.Annotations, managedDurationAnnotation)
		})
	}
}

func TestLiteLLMVirtualKeyReconciler_ReconcileUpdatesSecretMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, testKeyInfoPath, r.URL.Path)
		writeTestKeyInfo(w)
	}))
	defer srv.Close()

	key := testVirtualKey()
	key.Spec.SecretAnnotations = map[string]string{testReflectorAnnotation: testReflectorAllowed}
	key.Spec.SecretLabels = map[string]string{"tier": "backend"}
	proxy := testVirtualKeyProxy(srv.URL)
	masterKey := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace}, Data: map[string][]byte{testMasterSecretKey: []byte("master")}}
	output := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Spec.SecretName,
			Namespace: key.Namespace,
			// A stale managed key plus metadata written by another controller, which
			// the operator must leave alone.
			Annotations: map[string]string{
				"stale":                         "yes",
				managedAnnotationKeysAnnotation: "stale",
				"reflector.v1.k8s.emberstack.com/reflected-version": "42",
			},
			Labels: map[string]string{"owner": "someone-else"},
		},
		Data: map[string][]byte{key.SecretDataKey(): []byte("sk-generated")},
	}
	r := testVirtualKeyReconciler(t, key, proxy, masterKey, output)
	require.NoError(t, ctrl.SetControllerReference(key, output, r.Scheme))
	require.NoError(t, r.Update(t.Context(), output))

	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name}})
	require.NoError(t, err)

	var secret corev1.Secret
	require.NoError(t, r.Get(t.Context(), types.NamespacedName{Namespace: key.Namespace, Name: key.Spec.SecretName}, &secret))
	assert.Equal(t, testReflectorAllowed, secret.Annotations[testReflectorAnnotation])
	assert.Equal(t, "backend", secret.Labels["tier"])
	assert.NotContains(t, secret.Annotations, "stale", "a key dropped from the spec is removed")
	assert.Equal(t, "42", secret.Annotations["reflector.v1.k8s.emberstack.com/reflected-version"], "another controller's annotation survives")
	assert.Equal(t, "someone-else", secret.Labels["owner"])
	assert.Equal(t, []byte("sk-generated"), secret.Data[key.SecretDataKey()], "the key is not rotated")
}

func TestApplySecretMetadata(t *testing.T) {
	tests := []struct {
		name            string
		annotations     map[string]string
		labels          map[string]string
		specAnnotations map[string]string
		specLabels      map[string]string
		wantChanged     bool
		wantAnnotations map[string]string
		wantLabels      map[string]string
	}{
		{
			name:        "no metadata declared",
			annotations: map[string]string{testUnmanagedKey: testUnmanagedValue},
			wantLabels:  nil,
			wantAnnotations: map[string]string{
				testUnmanagedKey: testUnmanagedValue,
			},
		},
		{
			name:            "already applied",
			annotations:     map[string]string{"a": "1", managedAnnotationKeysAnnotation: "a"},
			specAnnotations: map[string]string{"a": "1"},
			wantAnnotations: map[string]string{"a": "1", managedAnnotationKeysAnnotation: "a"},
		},
		{
			name:            "value changed",
			annotations:     map[string]string{"a": "1", managedAnnotationKeysAnnotation: "a"},
			specAnnotations: map[string]string{"a": "2"},
			wantChanged:     true,
			wantAnnotations: map[string]string{"a": "2", managedAnnotationKeysAnnotation: "a"},
		},
		{
			name:            "all managed keys removed",
			annotations:     map[string]string{"a": "1", testUnmanagedKey: testUnmanagedValue, managedAnnotationKeysAnnotation: "a"},
			wantChanged:     true,
			wantAnnotations: map[string]string{testUnmanagedKey: testUnmanagedValue},
		},
		{
			name:        "unmanaged keys are never adopted",
			annotations: map[string]string{"a": testForeignValue},
			wantAnnotations: map[string]string{
				"a": testForeignValue,
			},
		},
		{
			// The CRD's CEL rules reject a reserved key at admission, so only an
			// object stored before those rules existed reaches here: the tracker
			// keeps owning its own name, and the reconcile stays idempotent.
			name:            "reserved spec key loses to the tracker it collides with",
			specAnnotations: map[string]string{managedAnnotationKeysAnnotation: testForeignValue, "a": "1"},
			wantChanged:     true,
			wantAnnotations: map[string]string{
				"a":                             "1",
				managedAnnotationKeysAnnotation: "a," + managedAnnotationKeysAnnotation,
			},
		},
		{
			name:            "labels tracked separately from annotations",
			specAnnotations: map[string]string{"a": "1"},
			specLabels:      map[string]string{"b": "2", "c": "3"},
			wantChanged:     true,
			wantAnnotations: map[string]string{
				"a":                             "1",
				managedAnnotationKeysAnnotation: "a",
				managedLabelKeysAnnotation:      "b,c",
			},
			wantLabels: map[string]string{"b": "2", "c": "3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations, Labels: tt.labels}}
			key := testVirtualKey()
			key.Spec.SecretAnnotations = tt.specAnnotations
			key.Spec.SecretLabels = tt.specLabels

			assert.Equal(t, tt.wantChanged, applySecretMetadata(secret, key))
			assert.Equal(t, tt.wantAnnotations, secret.Annotations)
			assert.Equal(t, tt.wantLabels, secret.Labels)
			assert.False(t, applySecretMetadata(secret, key), "a second apply is a no-op")
		})
	}
}

func TestLiteLLMVirtualKeyReconciler_ReconcileRejectsReservedSecretMetadata(t *testing.T) {
	tests := []struct {
		name         string
		reservedKey  string
		inLabels     bool
		secretExists bool
		wantField    string
	}{
		{
			name:        "tracker annotation declared as an annotation, Secret absent",
			reservedKey: managedAnnotationKeysAnnotation,
			wantField:   testAnnotationsSpec,
		},
		{
			name:         "tracker annotation declared as an annotation, Secret exists",
			reservedKey:  managedAnnotationKeysAnnotation,
			secretExists: true,
			wantField:    testAnnotationsSpec,
		},
		{
			name:        "unassigned reserved key declared as a label, Secret absent",
			reservedKey: managedKeysPrefix + "future",
			inLabels:    true,
			wantField:   testLabelsSpec,
		},
		{
			name:         "tracker annotation declared as a label, Secret exists",
			reservedKey:  managedLabelKeysAnnotation,
			inLabels:     true,
			secretExists: true,
			wantField:    testLabelsSpec,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				assert.Fail(t, "unexpected admin API call", r.URL.Path)
			}))
			defer srv.Close()

			key := testVirtualKey()
			reserved := map[string]string{tt.reservedKey: "user-value"}
			if tt.inLabels {
				key.Spec.SecretLabels = reserved
			} else {
				key.Spec.SecretAnnotations = reserved
			}
			proxy := testVirtualKeyProxy(srv.URL)
			masterKey := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace}, Data: map[string][]byte{testMasterSecretKey: []byte("master")}}
			objects := []runtime.Object{key, proxy, masterKey}
			output := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        key.Spec.SecretName,
					Namespace:   key.Namespace,
					Annotations: map[string]string{testUnmanagedKey: testUnmanagedValue},
					Labels:      map[string]string{testUnmanagedKey: testUnmanagedValue},
				},
				Data: map[string][]byte{key.SecretDataKey(): []byte(testGeneratedKey)},
			}
			if tt.secretExists {
				objects = append(objects, output)
			}
			r := testVirtualKeyReconciler(t, objects...)
			if tt.secretExists {
				require.NoError(t, ctrl.SetControllerReference(key, output, r.Scheme))
				require.NoError(t, r.Update(t.Context(), output))
			}

			_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name}})
			require.ErrorContains(t, err, "InvalidSpec")
			require.ErrorContains(t, err, tt.wantField)
			require.ErrorContains(t, err, tt.reservedKey)

			var secret corev1.Secret
			getErr := r.Get(t.Context(), types.NamespacedName{Namespace: key.Namespace, Name: key.Spec.SecretName}, &secret)
			if !tt.secretExists {
				assert.True(t, apierrors.IsNotFound(getErr), "no Secret is created for an invalid spec")
				return
			}
			require.NoError(t, getErr)
			assert.Equal(t, map[string]string{testUnmanagedKey: testUnmanagedValue}, secret.Annotations, "existing Secret metadata is left alone")
			assert.Equal(t, map[string]string{testUnmanagedKey: testUnmanagedValue}, secret.Labels)

			var reconciled litellmv1alpha1.LiteLLMVirtualKey
			require.NoError(t, r.Get(t.Context(), types.NamespacedName{Namespace: key.Namespace, Name: key.Name}, &reconciled))
			condition := meta.FindStatusCondition(reconciled.Status.Conditions, conditionTypeReady)
			require.NotNil(t, condition)
			assert.Equal(t, metav1.ConditionFalse, condition.Status)
			assert.Equal(t, "InvalidSpec", condition.Reason)
		})
	}
}

// A spec the guard rejects must still be able to release its remote key, so
// reconcileDelete never consults the reserved-key guard.
func TestLiteLLMVirtualKeyReconciler_ReconcileDeleteIgnoresReservedSecretMetadata(t *testing.T) {
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/key/delete", r.URL.Path)
		var request struct {
			Keys []string `json:"keys"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		deleted = request.Keys
	}))
	defer srv.Close()

	key := testVirtualKey()
	key.Finalizers = []string{virtualKeyFinalizer}
	key.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	key.Spec.SecretAnnotations = map[string]string{managedAnnotationKeysAnnotation: "user-value"}
	proxy := testVirtualKeyProxy(srv.URL)
	masterKey := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace}, Data: map[string][]byte{testMasterSecretKey: []byte("master")}}
	output := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: key.Spec.SecretName, Namespace: key.Namespace},
		Data:       map[string][]byte{key.SecretDataKey(): []byte(testGeneratedKey)},
	}
	r := testVirtualKeyReconciler(t, key, proxy, masterKey, output)

	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name}})
	require.NoError(t, err)
	assert.Equal(t, []string{testGeneratedKey}, deleted)
}

func TestLiteLLMVirtualKeyReconciler_ReconcileDeleteDeletesRemoteKey(t *testing.T) {
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/key/delete", r.URL.Path)
		var request struct {
			Keys []string `json:"keys"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		deleted = request.Keys
	}))
	defer srv.Close()

	key := testVirtualKey()
	key.Finalizers = []string{virtualKeyFinalizer}
	proxy := testVirtualKeyProxy(srv.URL)
	masterKey := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testMasterSecretName, Namespace: key.Namespace},
		Data:       map[string][]byte{testMasterSecretKey: []byte("master")},
	}
	output := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: key.Spec.SecretName, Namespace: key.Namespace},
		Data:       map[string][]byte{key.SecretDataKey(): []byte("sk-generated")},
	}
	r := testVirtualKeyReconciler(t, key, proxy, masterKey, output)

	require.NoError(t, r.reconcileDelete(t.Context(), key))
	assert.Equal(t, []string{"sk-generated"}, deleted)
	assert.NotContains(t, key.Finalizers, virtualKeyFinalizer)
}

func TestVirtualKeyRequest_MaxBudget(t *testing.T) {
	tests := []struct {
		name      string
		maxBudget string
		want      float64
		wantErr   string
	}{
		{name: "omitted"},
		{name: "decimal", maxBudget: "12.50", want: 12.5},
		{name: "invalid", maxBudget: "twelve", wantErr: "parse maxBudget"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := testVirtualKey()
			key.Spec.MaxBudget = tt.maxBudget
			request, err := virtualKeyRequest(key)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.maxBudget == "" {
				assert.Nil(t, request.MaxBudget)
				return
			}
			require.NotNil(t, request.MaxBudget)
			assert.Equal(t, tt.want, *request.MaxBudget)
		})
	}
}

func testVirtualKey() *litellmv1alpha1.LiteLLMVirtualKey {
	return &litellmv1alpha1.LiteLLMVirtualKey{
		ObjectMeta: metav1.ObjectMeta{Name: "application", Namespace: "default"},
		Spec: litellmv1alpha1.LiteLLMVirtualKeySpec{
			ProxyRef:   testProxyName,
			SecretName: "application-key",
			SecretKey:  "token",
			KeyAlias:   "application",
			Models:     []string{"openai/gpt-5"},
			TeamID:     "team-a",
		},
	}
}

func testVirtualKeyProxy(endpoint string) *litellmv1alpha1.LiteLLMProxy {
	return &litellmv1alpha1.LiteLLMProxy{
		ObjectMeta: metav1.ObjectMeta{Name: testProxyName, Namespace: "default"},
		Spec: litellmv1alpha1.LiteLLMProxySpec{APIAccess: &litellmv1alpha1.APIAccessSpec{
			Endpoint:     endpoint,
			MasterKeyRef: litellmv1alpha1.SecretKeyRef{Name: testMasterSecretName, Key: testMasterSecretKey},
		}},
	}
}

func testVirtualKeyReconciler(t *testing.T, objects ...runtime.Object) *LiteLLMVirtualKeyReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, litellmv1alpha1.AddToScheme(scheme))
	return &LiteLLMVirtualKeyReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&litellmv1alpha1.LiteLLMVirtualKey{}).WithRuntimeObjects(objects...).Build(),
		Scheme: scheme,
	}
}

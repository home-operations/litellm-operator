package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

var _ = Describe("MCP API credential rotation", func() {
	It("watches a foreign Secret and retries a failed update without changing the pod template", func() {
		var mu sync.Mutex
		var registered []map[string]any
		var appliedToken string
		failUpdate := false
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			switch {
			case req.URL.Path == "/model/info":
				_, _ = w.Write([]byte(`{"data":[]}`))
			case req.URL.Path == "/v1/mcp/server" && req.Method == http.MethodGet:
				if registered == nil {
					registered = []map[string]any{}
				}
				_ = json.NewEncoder(w).Encode(registered)
			case req.URL.Path == "/v1/mcp/server":
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				credentials, _ := body["credentials"].(map[string]any)
				token, _ := credentials["auth_value"].(string)
				if failUpdate {
					http.Error(w, token, http.StatusServiceUnavailable)
					return
				}
				appliedToken = token
				body["credentials"] = nil
				registered = []map[string]any{body}
				_, _ = w.Write([]byte(`{}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		DeferCleanup(api.Close)
		for _, namespace := range []string{"mcp-api-ai", "mcp-api-apps"} {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
		}
		proxy := &litellmv1alpha1.LiteLLMProxy{
			ObjectMeta: metav1.ObjectMeta{Name: "gateway", Namespace: "mcp-api-ai"},
			Spec: litellmv1alpha1.LiteLLMProxySpec{ApplyMode: "api", APIAccess: &litellmv1alpha1.APIAccessSpec{
				Endpoint: api.URL, MasterKeyRef: litellmv1alpha1.SecretKeyRef{Name: "master", Key: "key"},
			}},
		}
		Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "master", Namespace: proxy.Namespace},
			Data: map[string][]byte{"key": []byte("test-master")}})).To(Succeed())
		source := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "installation", Namespace: "mcp-api-apps"},
			Data: map[string][]byte{"token": []byte("first-installation-token")}}
		Expect(k8sClient.Create(ctx, source)).To(Succeed())
		server := &litellmv1alpha1.LiteLLMMCPServer{
			ObjectMeta: metav1.ObjectMeta{Name: "github", Namespace: source.Namespace},
			Spec: litellmv1alpha1.LiteLLMMCPServerSpec{ProxyRef: proxy.Name, ProxyNamespace: proxy.Namespace,
				URL: "https://github.example/mcp", Transport: "http", AuthType: "bearer_token",
				AuthTokenRef: &litellmv1alpha1.SecretKeyRef{Name: source.Name, Key: "token"}},
		}
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		Expect(k8sClient.Create(ctx, proxy)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, proxy)).To(Succeed()) })
		key := types.NamespacedName{Name: proxy.Name, Namespace: proxy.Namespace}
		Eventually(func(g Gomega) {
			mu.Lock()
			defer mu.Unlock()
			g.Expect(appliedToken).To(Equal("first-installation-token"))
			g.Expect(k8sClient.Get(ctx, key, proxy)).To(Succeed())
			g.Expect(proxy.Status.MCPConfigHash).NotTo(BeEmpty())
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		var before appsv1.Deployment
		Expect(k8sClient.Get(ctx, key, &before)).To(Succeed())
		mu.Lock()
		failUpdate = true
		mu.Unlock()
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: source.Name, Namespace: source.Namespace}, source)).To(Succeed())
		source.Data["token"] = []byte("rotated-installation-token")
		Expect(k8sClient.Update(ctx, source)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, proxy)).To(Succeed())
			condition := meta.FindStatusCondition(proxy.Status.Conditions, "Ready")
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal("MCPAPISyncFailed"))
			g.Expect(condition.Message).NotTo(ContainSubstring("rotated-installation-token"))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		mu.Lock()
		failUpdate = false
		mu.Unlock()
		Eventually(func(g Gomega) {
			mu.Lock()
			defer mu.Unlock()
			g.Expect(appliedToken).To(Equal("rotated-installation-token"))
			g.Expect(k8sClient.Get(ctx, key, proxy)).To(Succeed())
			g.Expect(meta.IsStatusConditionTrue(proxy.Status.Conditions, "Ready")).To(BeTrue())
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		var after appsv1.Deployment
		Expect(k8sClient.Get(ctx, key, &after)).To(Succeed())
		Expect(after.Spec.Template).To(Equal(before.Spec.Template))
		Expect(after.Generation).To(Equal(before.Generation))
	})
})

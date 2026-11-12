package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Secret cache", func() {
	It("leaves Helm release Secrets out", func() {
		release := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.app.v1", Namespace: metav1.NamespaceDefault},
			Type:       "helm.sh/release.v1",
		}
		opaque := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "cache-opaque", Namespace: metav1.NamespaceDefault},
		}
		// Watch events arrive in order, so the release would be cached before the later Secret.
		Expect(k8sClient.Create(ctx, release)).To(Succeed())
		Expect(k8sClient.Create(ctx, opaque)).To(Succeed())

		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(opaque), &corev1.Secret{})
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
		err := mgrClient.Get(ctx, client.ObjectKeyFromObject(release), &corev1.Secret{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "got %v", err)
	})
})

package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const helmReleaseSecretType = "helm.sh/release.v1"

// CacheOptions leaves Helm release Secrets out of the cluster-wide Secret
// informer: they hold whole rendered charts and nothing can reference them.
func CacheOptions() cache.Options {
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Secret{}: {Field: fields.OneTermNotEqualSelector("type", helmReleaseSecretType)},
		},
	}
}

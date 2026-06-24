package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const configHashAnnotation = "litellm.home-operations.com/config-hash"

func configMapName(proxy *litellmv1alpha1.LiteLLMProxy) string {
	return proxy.Name + "-config"
}

func selectorLabels(proxy *litellmv1alpha1.LiteLLMProxy) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "litellm",
		"app.kubernetes.io/instance":   proxy.Name,
		"app.kubernetes.io/managed-by": "litellm-operator",
	}
}

func buildConfigMap(proxy *litellmv1alpha1.LiteLLMProxy, config string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(proxy),
			Namespace: proxy.Namespace,
			Labels:    selectorLabels(proxy),
		},
		Data: map[string]string{configFileName: config},
	}
}

func buildService(proxy *litellmv1alpha1.LiteLLMProxy) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxy.Name,
			Namespace: proxy.Namespace,
			Labels:    selectorLabels(proxy),
		},
		Spec: corev1.ServiceSpec{
			Type:     proxy.Spec.Service.Type,
			Selector: selectorLabels(proxy),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       proxy.Spec.Service.Port,
				TargetPort: intstr.FromInt32(proxyPort),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

func defaultProbe(path string) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt32(proxyPort)},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

func buildDeployment(proxy *litellmv1alpha1.LiteLLMProxy, configHash string, modelEnv []corev1.EnvVar) *appsv1.Deployment {
	labels := selectorLabels(proxy)
	env := make([]corev1.EnvVar, 0, len(proxy.Spec.Env)+len(modelEnv))
	env = append(env, proxy.Spec.Env...)
	env = append(env, modelEnv...)

	liveness := proxy.Spec.LivenessProbe
	if liveness == nil {
		liveness = defaultProbe("/health/liveliness")
	}
	readiness := proxy.Spec.ReadinessProbe
	if readiness == nil {
		readiness = defaultProbe("/health/readiness")
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxy.Name,
			Namespace: proxy.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: proxy.Spec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: map[string]string{configHashAnnotation: configHash},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    proxyContainer,
						Image:   proxy.Spec.Image,
						Args:    []string{"--config", fmt.Sprintf("%s/%s", configMountPath, configFileName), "--port", fmt.Sprintf("%d", proxyPort)},
						Env:     env,
						EnvFrom: proxy.Spec.EnvFrom,
						Ports: []corev1.ContainerPort{{
							Name:          "http",
							ContainerPort: proxyPort,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources:      proxy.Spec.Resources,
						LivenessProbe:  liveness,
						ReadinessProbe: readiness,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "config",
							MountPath: configMountPath,
							ReadOnly:  true,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: configMapName(proxy)},
							},
						},
					}},
				},
			},
		},
	}
}

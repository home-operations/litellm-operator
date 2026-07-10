package controller

import (
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const configHashAnnotation = "litellm.home-operations.com/config-hash"

func configMapName(proxy *litellmv1alpha1.LiteLLMProxy) string {
	return proxy.Name + "-config"
}

func selectorLabels(proxy *litellmv1alpha1.LiteLLMProxy) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       appName,
		"app.kubernetes.io/instance":   proxy.Name,
		"app.kubernetes.io/managed-by": "litellm-operator",
	}
}

// podLabels merges user-supplied pod labels under the operator-managed selector
// labels, which always win so the Service keeps matching the pod.
func podLabels(selector, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return selector
	}
	merged := make(map[string]string, len(selector)+len(extra))
	maps.Copy(merged, extra)
	maps.Copy(merged, selector)
	return merged
}

// podAnnotations merges user-supplied pod annotations under the operator-managed
// config-hash annotation, which always wins so config changes still roll the pods.
func podAnnotations(extra map[string]string, configHash string) map[string]string {
	merged := make(map[string]string, len(extra)+1)
	maps.Copy(merged, extra)
	merged[configHashAnnotation] = configHash
	return merged
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

func servicePort(proxy *litellmv1alpha1.LiteLLMProxy) int32 {
	if proxy.Spec.Service.Port != 0 {
		return proxy.Spec.Service.Port
	}
	return proxyPort
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
				Name:       httpPortName,
				Port:       servicePort(proxy),
				TargetPort: intstr.FromInt32(proxyPort),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

func buildRoute(proxy *litellmv1alpha1.LiteLLMProxy) *gatewayv1.HTTPRoute {
	route := proxy.Spec.Route

	parentRefs := make([]gatewayv1.ParentReference, 0, len(route.ParentRefs))
	for _, p := range route.ParentRefs {
		ref := gatewayv1.ParentReference{Name: gatewayv1.ObjectName(p.Name)}
		if p.Namespace != "" {
			ns := gatewayv1.Namespace(p.Namespace)
			ref.Namespace = &ns
		}
		if p.SectionName != "" {
			sn := gatewayv1.SectionName(p.SectionName)
			ref.SectionName = &sn
		}
		parentRefs = append(parentRefs, ref)
	}

	hostnames := make([]gatewayv1.Hostname, 0, len(route.Hostnames))
	for _, h := range route.Hostnames {
		hostnames = append(hostnames, gatewayv1.Hostname(h))
	}

	port := servicePort(proxy)

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxy.Name,
			Namespace: proxy.Namespace,
			Labels:    selectorLabels(proxy),
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: parentRefs},
			Hostnames:       hostnames,
			Rules: []gatewayv1.HTTPRouteRule{{
				BackendRefs: []gatewayv1.HTTPBackendRef{{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(proxy.Name),
							Port: &port,
						},
					},
				}},
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

	volumeMounts := append([]corev1.VolumeMount{{
		Name:      configVolumeName,
		MountPath: configMountPath,
		ReadOnly:  true,
	}}, proxy.Spec.VolumeMounts...)

	volumes := append([]corev1.Volume{{
		Name: configVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName(proxy)},
			},
		},
	}}, proxy.Spec.Volumes...)

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
					Labels:      podLabels(labels, proxy.Spec.PodLabels),
					Annotations: podAnnotations(proxy.Spec.PodAnnotations, configHash),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    proxyContainer,
						Image:   proxy.Spec.Image,
						Args:    []string{"--config", fmt.Sprintf("%s/%s", configMountPath, configFileName), "--port", fmt.Sprintf("%d", proxyPort)},
						Env:     env,
						EnvFrom: proxy.Spec.EnvFrom,
						Ports: []corev1.ContainerPort{{
							Name:          httpPortName,
							ContainerPort: proxyPort,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources:      proxy.Spec.Resources,
						LivenessProbe:  liveness,
						ReadinessProbe: readiness,
						VolumeMounts:   volumeMounts,
					}},
					Volumes: volumes,
				},
			},
		},
	}
}

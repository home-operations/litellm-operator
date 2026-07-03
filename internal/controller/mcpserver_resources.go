package controller

import (
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	litellmv1alpha1 "github.com/home-operations/litellm-operator/api/v1alpha1"
)

const mcpContainer = "mcp"

func mcpSelectorLabels(server *litellmv1alpha1.LiteLLMMCPServer) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       server.Name,
		"app.kubernetes.io/managed-by": "litellm-operator",
		"app.kubernetes.io/component":  "mcp-server",
	}
}

// mcpPodLabels merges user-supplied pod labels under the operator-managed
// selector labels, which always win so the Service keeps matching the pod.
func mcpPodLabels(selector, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return selector
	}
	merged := make(map[string]string, len(selector)+len(extra))
	maps.Copy(merged, extra)
	maps.Copy(merged, selector)
	return merged
}

func buildMCPDeployment(server *litellmv1alpha1.LiteLLMMCPServer) *appsv1.Deployment {
	w := server.Spec.Workload
	labels := mcpSelectorLabels(server)
	port := server.WorkloadPort()

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: w.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      mcpPodLabels(labels, w.PodLabels),
					Annotations: w.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:           w.ServiceAccountName,
					AutomountServiceAccountToken: w.AutomountServiceAccountToken,
					Containers: []corev1.Container{{
						Name:    mcpContainer,
						Image:   w.Image,
						Command: w.Command,
						Args:    w.Args,
						Env:     w.Env,
						EnvFrom: w.EnvFrom,
						Ports: []corev1.ContainerPort{{
							Name:          httpPortName,
							ContainerPort: port,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources:       w.Resources,
						SecurityContext: w.SecurityContext,
						VolumeMounts:    w.VolumeMounts,
					}},
					SecurityContext: w.PodSecurityContext,
					NodeSelector:    w.NodeSelector,
					Tolerations:     w.Tolerations,
					Affinity:        w.Affinity,
					Volumes:         w.Volumes,
				},
			},
		},
	}
}

func buildMCPService(server *litellmv1alpha1.LiteLLMMCPServer) *corev1.Service {
	labels := mcpSelectorLabels(server)
	port := server.WorkloadPort()

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       httpPortName,
				Port:       port,
				TargetPort: intstr.FromInt32(port),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

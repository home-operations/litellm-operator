# litellm-operator

A Kubernetes operator that renders LiteLLM proxy config from CRDs

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.0](https://img.shields.io/badge/AppVersion-0.0.0-informational?style=flat-square)

## Installing

```sh
helm install litellm-operator oci://ghcr.io/home-operations/charts/litellm-operator \
  --namespace litellm-system --create-namespace
```

The chart installs the CRDs (`LiteLLMProxy`, `LiteLLMModel`) and the operator
Deployment. Declare a `LiteLLMProxy` plus one `LiteLLMModel` per model; the
operator renders `config.yaml` into a ConfigMap, wires secret-backed API keys as
`os.environ` env vars, and rolls the proxy whenever the rendered config changes.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity. |
| controller.leaderElection.enabled | bool | `true` | Enable leader election so only one replica reconciles at a time. |
| controller.logLevel | string | `"info"` | Log level (debug, info). |
| controller.metrics.annotations | object | `{}` | Annotations for the metrics Service. |
| controller.metrics.port | int | `8081` | Operational port: /metrics plus the /healthz and /readyz probes (plain HTTP; restrict with a NetworkPolicy if needed). |
| env | list | `[]` | Extra environment variables for the operator container. |
| fullnameOverride | string | `""` | Override the full release name. |
| image.digest | string | `""` | Pin the image by digest (sha256:…); when set, overrides the tag. |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy. |
| image.repository | string | `"ghcr.io/home-operations/litellm-operator"` | Image repository. |
| image.tag | string | `""` | Overrides the image tag; defaults to the chart appVersion. |
| imagePullSecrets | list | `[]` | Image pull secrets for private registries. |
| livenessProbe.httpGet.path | string | `"/healthz"` |  |
| livenessProbe.httpGet.port | string | `"metrics"` |  |
| livenessProbe.initialDelaySeconds | int | `15` |  |
| livenessProbe.periodSeconds | int | `20` |  |
| llmkube.autoRegister | bool | `false` | Auto-register LiteLLMModels from LLMKube (inference.llmkube.dev) InferenceServices. When a service reaches Ready, the operator creates a matching LiteLLMModel in the same namespace, which proxies adopt as usual. Adds RBAC for inference.llmkube.dev and sets ENABLE_LLMKUBE_AUTOREGISTER. No-op (logs a warning) if the LLMKube CRDs are not installed. |
| nameOverride | string | `""` | Override the chart name used in resource names. |
| nodeSelector | object | `{}` | Node selector. |
| podAnnotations | object | `{}` | Pod annotations. |
| podLabels | object | `{}` | Pod labels. |
| podSecurityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod-level security context. |
| priorityClassName | string | `""` | Priority class for the operator pod. |
| rbac.annotations | object | `{}` | Annotations for the RBAC resources. |
| rbac.create | bool | `true` | Create the ClusterRole/ClusterRoleBinding and leader-election Role. |
| readinessProbe.httpGet.path | string | `"/readyz"` |  |
| readinessProbe.httpGet.port | string | `"metrics"` |  |
| readinessProbe.initialDelaySeconds | int | `5` |  |
| readinessProbe.periodSeconds | int | `10` |  |
| replicaCount | int | `1` | Number of operator replicas (one is active at a time when leader election is on). |
| resources | object | `{"limits":{"memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | Operator container resource requests/limits. |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container-level security context. |
| serviceAccount.annotations | object | `{}` | Annotations for the ServiceAccount. |
| serviceAccount.automount | bool | `true` | Automount the API token. |
| serviceAccount.create | bool | `true` | Create a ServiceAccount. |
| serviceAccount.name | string | `""` | Name of the ServiceAccount; generated when empty. |
| tolerations | list | `[]` | Tolerations. |
| webhook.enabled | bool | `true` | Enable the validating admission webhook. The operator self-manages the serving certificate (no cert-manager dependency) and patches the CA bundle into the ValidatingWebhookConfiguration. |
| webhook.port | int | `9443` | Webhook server container port. |


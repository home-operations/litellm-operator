# litellm-operator

A Kubernetes operator that turns a [LiteLLM](https://github.com/BerriAI/litellm)
proxy config into first-class, API-server-validated resources. Instead of
hand-editing one large `config.yaml` ConfigMap, you declare a `LiteLLMProxy` and
one `LiteLLMModel` per model; the operator renders the config, wires secrets,
and rolls the proxy on change.

## How it works

A `LiteLLMProxy` owns a Deployment, Service, and ConfigMap. Its `modelSelector`
matches `LiteLLMModel` resources in the same namespace. On every change the
operator:

- renders `config.yaml` deterministically (models sorted by name, so the output
  and its hash are stable),
- sources each `apiKeyRef` from a Secret as an `os.environ/...` env var on the
  Deployment, so secret values never land in the ConfigMap,
- stamps a `litellm.home-operations.com/config-hash` annotation on the pod
  template so the proxy performs a rolling restart only when the config actually
  changes.

## Example

```yaml
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMProxy
metadata:
  name: main
  namespace: ai
spec:
  modelSelector:
    matchLabels:
      litellm.home-operations.com/proxy: main
  routerSettings:
    routing_strategy: simple-shuffle
  envFrom:
    - secretRef:
        name: litellm-secrets
---
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMModel
metadata:
  name: glm-5-2
  namespace: ai
  labels:
    litellm.home-operations.com/proxy: main
spec:
  modelName: glm-5.2
  params:
    model: openai/glm-5.2
    apiBase: https://api.z.ai/api/coding/paas/v4
    apiKeyRef:
      name: litellm-secrets
      key: ZAI_API_KEY
    dropParams: true
  info:
    max_input_tokens: 1000000
    supports_function_calling: true
```

The typed fields cover the common `litellm_params`; anything else (provider
quirks) goes under `params.additional`, and `info` maps verbatim to
`model_info`.

## Install

```sh
helm install litellm-operator oci://ghcr.io/home-operations/charts/litellm-operator \
  --namespace litellm-system --create-namespace
```

## Development

Tooling is pinned with [mise](https://mise.jdx.dev); run `mise install` once,
then:

```sh
mise run test              # unit tests
mise run test-integration  # envtest integration tests
mise run lint              # golangci-lint
mise run build             # build the manager binary
mise run run               # run the controller against your kubeconfig
```

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
  routerSettings:
    routing_strategy: simple-shuffle
  # No modelSelector: this proxy adopts every LiteLLMModel in its namespace.
  route:
    hostnames:
      - litellm.example.com
    parentRefs:
      - name: envoy-external
        namespace: network
---
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMModel
metadata:
  name: glm-5-2
  namespace: ai
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
    maxInputTokens: 1000000
    supportsFunctionCalling: true
```

Models bind to a proxy in one of three ways, most specific first: a model's
`spec.proxyRef` names its proxy explicitly; otherwise a proxy's
`spec.modelSelector` matches model labels; otherwise a proxy with no selector
adopts every model in its namespace. `info` fields are typed and validated, with
`info.extra` and `params.additional` as escape hatches for the long tail.
`apiKeyRef`/`apiBaseRef` source values from a Secret (the operator wires the env
var and keeps them out of the rendered config); `apiKey`/`apiBase` take literals.
When `spec.route` is set, the operator creates and owns a Gateway API HTTPRoute
fronting the proxy Service; the Gateway API CRDs are only required if you use it.

## Validation

A validating admission webhook (enabled by default) rejects mistakes before they
reach the cluster: a `LiteLLMModel` that sets both `apiKey` and `apiKeyRef`, two
models whose names sanitize to the same injected env var (which would silently
clobber one key), and a `LiteLLMProxy` whose `spec.route` is missing hostnames or
a parent reference. The operator self-manages the webhook serving certificate and
patches the CA bundle into the `ValidatingWebhookConfiguration`, so there is no
cert-manager dependency. Set `webhook.enabled=false` to turn it off.

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
mise run test-e2e          # kind-based end-to-end tests
mise run lint              # golangci-lint
mise run build             # build the manager binary
mise run run               # run the controller against your kubeconfig
```

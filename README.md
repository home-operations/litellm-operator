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
      # Other parent kinds work where the Gateway implementation supports them.
      # A Service parent needs group: "" (the core API group) set explicitly.
      - group: ""
        kind: Service
        name: main
        port: 4000
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

Models, guardrails, MCP servers, teams, and virtual keys can reference a proxy
in another namespace using `spec.proxyNamespace` alongside `spec.proxyRef`:

```yaml
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: application
  namespace: apps
spec:
  proxyRef: main
  proxyNamespace: ai
  secretName: application-key
```

Omitting `proxyNamespace` uses the resource's own namespace. Selectors and
default adoption remain limited to the proxy's namespace. Generated virtual-key
Secrets and MCP workloads stay in their resource's namespace. The proxy's master
key and Secret references injected into its Deployment (`apiKeyRef`,
`apiBaseRef`, and `authTokenRef`) must exist in the proxy's namespace; Secret
references inside an MCP workload use the MCP server's namespace.

Guardrails and MCP servers are their own CRDs — `LiteLLMGuardrail` and
`LiteLLMMCPServer` — adopted by a proxy the same way models are (proxyRef,
selector, or namespace default) and rendered into the proxy's `guardrails` list
and `mcp_servers` map, with the same `apiKeyRef`/`authTokenRef` secret wiring. A
`LiteLLMMCPServer` either points the gateway at an external `spec.url`, or sets
`spec.workload` (image, port, env, volumes, ...) to have the operator run the
server itself as a Deployment + Service and derive the url from it
(`http://<name>.<namespace>.svc.cluster.local:<port><path>`, surfaced in
`status.resolvedURL`). A validating webhook enforces that exactly one of `url`
or `workload` is set.
Callbacks are a typed proxy field (`spec.callbacks` → `success_callback`,
`failure_callback`, `callbacks`, and the top-level `callback_settings`). Every
other top-level litellm config key has a named field on the proxy
(`environmentVariables`, `credentialList`, `defaultVertexConfig`, `filesSettings`,
`assistantSettings`, `finetuneSettings`, `prompts`, `vectorStoreRegistry`), and
`spec.extraConfig` is a final top-level catch-all for anything litellm adds
later. The generated `model_list`, `guardrails`, `mcp_servers` and the typed
blocks take precedence over `extraConfig`. The three settings blocks
(`generalSettings`, `routerSettings`, `litellmSettings`) remain free-form
passthroughs.

## Workload customization

`podAnnotations` and `podLabels` land on the pod template; the operator's
config-hash annotation and selector labels take precedence. `volumes` and
`volumeMounts` take standard `corev1` shapes and are merged alongside the
operator's `config` volume — the reserved name `config` and the `/etc/litellm`
mount path are rejected at admission so they can't be shadowed.

For example, LiteLLM's ChatGPT OAuth provider needs its device-flow refresh token
to survive restarts and be shared by every replica, so it goes on a PVC:

```yaml
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMProxy
metadata:
  name: main
  namespace: ai
spec:
  replicas: 3
  env:
    - name: CHATGPT_TOKEN_DIR
      value: /app/chatgpt_tokens
  podAnnotations:
    reloader.stakater.com/auto: "true"
  volumes:
    - name: chatgpt-tokens
      persistentVolumeClaim:
        claimName: litellm
  volumeMounts:
    - name: chatgpt-tokens
      mountPath: /app/chatgpt_tokens
```

The PVC (`litellm` here) must exist and, for multi-replica proxies, use an access
mode that allows shared read-write (e.g. `ReadWriteMany`).

## Apply modes

`spec.applyMode` controls how models reach the proxy. The default, `file`, renders
everything into `config.yaml` and rolls the Deployment on change — declarative,
GitOps-friendly, and Redis-only (no database). `api` instead pushes models to the
proxy's DB-backed admin API live, with no restart; it requires the proxy to run
in DB mode (Postgres) and `spec.apiAccess.masterKeyRef` to authenticate. In api
mode guardrails, MCP servers and settings still render into `config.yaml` (with
`store_model_in_db: true`); only the volatile `model_list` goes over the API.
Secret-backed keys keep the `os.environ/...` indirection in both modes, so the
operator wires env vars onto the Deployment and never reads secret values itself.
The operator only manages models it created (tagged in `model_info`), leaving
UI- or hand-added models alone.

## Teams

`LiteLLMTeam.spec.mcpServers` assigns MCP server IDs or aliases to a team through
LiteLLM's `object_permission.mcp_servers` field. Spec changes update the team in
place. Removing the field or setting it to `[]` clears team-level server
assignments while preserving other object permissions. Use LiteLLM's
`no-mcp-servers` sentinel to deny MCP access.

```yaml
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMTeam
metadata:
  name: apps
  namespace: ai
spec:
  proxyRef: main
  mcpServers:
    - ha-mcp
    - context7
```

## Virtual keys

A `LiteLLMVirtualKey` mints a key through the proxy's admin API and writes it to a
Secret it owns in the same namespace, deleting the remote key when the resource
goes away. Spec changes update the remote key in place, including cleared settings.
Expiry changes only when `duration` changes. Existing keys apply their duration
once on upgrade.

`spec.mcpToolsets` assigns MCP toolset IDs to the key through LiteLLM's
`object_permission.mcp_toolsets` field. Reconciliation restores assignments that
drift. Removing the field or setting it to `[]` clears key-level toolset
assignments; LiteLLM's other MCP permissions and inheritance rules still apply.

```yaml
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: hermes
  namespace: ai
spec:
  proxyRef: main
  secretName: hermes-key
  models:
    - gpt-5.6-terra
  mcpToolsets:
    - litellm # Use the toolset ID returned by LiteLLM's /v1/mcp/toolset API.
```

`secretAnnotations` and `secretLabels` land on that Secret, so it can
carry metadata other controllers act on — notably kubernetes-reflector's
`reflection-allowed`, which has to sit on the _source_ Secret for the key to be
mirrored into another namespace:

```yaml
apiVersion: litellm.home-operations.com/v1alpha1
kind: LiteLLMVirtualKey
metadata:
  name: application
  namespace: ai
spec:
  proxyRef: main
  secretName: application-key
  keyAlias: application
  secretAnnotations:
    reflector.v1.k8s.emberstack.com/reflection-allowed: "true"
    reflector.v1.k8s.emberstack.com/reflection-allowed-namespaces: apps
    reflector.v1.k8s.emberstack.com/reflection-auto-enabled: "true"
```

The operator manages only the keys the spec names: metadata written by anything
else — reflector's own bookkeeping on the mirror, another controller's
annotations — is left alone, and a key dropped from the spec is removed from the
Secret on the next reconcile. It tells the two apart by recording the keys it
applied in `litellm.home-operations.com/managed-annotations` and
`.../managed-labels` on the Secret. The `litellm.home-operations.com/managed-`
prefix is reserved for that bookkeeping: the CRD rejects a `secretAnnotations` or
`secretLabels` key using it. Both maps reconcile in place, so editing them
neither rotates the key nor touches the Secret's data.

## Validation

A validating admission webhook (enabled by default) rejects mistakes before they
reach the cluster: a `LiteLLMModel` that sets both `apiKey` and `apiKeyRef`, two
models whose names sanitize to the same injected env var (which would silently
clobber one key), a `LiteLLMProxy` whose `spec.route` is missing hostnames or
a parent reference, and a `LiteLLMProxy` whose `spec.volumes`/`spec.volumeMounts`
reuse the reserved `config` name or the `/etc/litellm` mount path. The operator self-manages the webhook serving certificate and
patches the CA bundle into the `ValidatingWebhookConfiguration`, so there is no
cert-manager dependency. Set `webhook.enabled=false` to turn it off.

## LLMKube auto-registration

The operator can mirror in-cluster models served by
[LLMKube](https://github.com/defilantech/llmkube) into `LiteLLMModel` resources,
so a model deployed as an LLMKube `InferenceService` shows up on the proxy
without you writing a `LiteLLMModel` by hand. It is opt-in: set
`ENABLE_LLMKUBE_AUTOREGISTER=1` (Helm: `llmkube.autoRegister=true`). The flag is
the toggle; a startup discovery check is the safety net — if the
`inference.llmkube.dev` CRDs are not installed the operator logs a warning and
skips the feature rather than failing, so enabling it early is harmless. The
CRDs are only required when the flag is on.

When an `InferenceService` reaches `status.phase=Ready` with an endpoint, the
operator creates a `LiteLLMModel` named after it, **in the same namespace**
(ownership and proxy adoption both require co-location). The model is owned by the
`InferenceService`, so Kubernetes garbage-collects it when the service is
deleted. The projection only asserts what LLMKube can report truthfully:

- `params.model` is `openai/<modelRef>` — all LLMKube runtimes serve an
  OpenAI-compatible API, and `params.apiBase` is the service endpoint trimmed to
  its `/v1` root;
- `params.apiKey` is a placeholder (`sk-llmkube-noauth`) because the endpoint is
  unauthenticated, but litellm's `openai` provider still requires a non-empty key;
- `info.maxInputTokens` comes from the Model's parsed GGUF context length when
  available — capability flags litellm cannot infer (function calling, vision,
  prompt caching) are left unset rather than guessed;
- `info.mode` is set for embedding and reranker models. LLMKube has no task-type
  field, so the mode is inferred from the runtime flags you already set
  (`--reranking` → `rerank`, `--embedding` → `embedding`; rerank wins when both
  are present, as llama.cpp rerankers pass both), falling back to the endpoint
  path (`/v1/rerank`, `/v1/embeddings`). Override it explicitly with the
  `litellm.home-operations.com/mode` annotation on the InferenceService when the
  heuristic can't tell (e.g. a generic runtime). Plain chat models get no mode.

The generated model carries `litellm.home-operations.com/managed-by: llmkube`, so
a proxy can target these models specifically via `modelSelector`, and the
operator never overwrites a same-named model it does not own (it logs and skips).
While auto-registration is enabled, the model is removed when the
`InferenceService` reaches a terminal phase
(`Failed`/`Stopped`); transient dips (`Progressing`, a rolling update) leave it in
place so a routine pod rollout does not churn the proxy config. A complete
example is in `config/samples/llmkube_autoregister.yaml`. Installing the LLMKube
CRDs after the operator is running requires an operator restart to pick them up.

Setting `llmkube.autoRegister=false` removes the auto-registered `LiteLLMModel`
resources after the operator restarts. Manually managed models and the source
`InferenceService` resources remain in place. Cleanup also works without the
LLMKube CRDs installed.

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

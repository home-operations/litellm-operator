# Changelog

## [0.0.6](https://github.com/home-operations/litellm-operator/compare/0.0.5...0.0.6) (2026-07-04)


### Bug Fixes

* **deps:** update module github.com/defilantech/llmkube (v0.8.26 → v0.8.28) ([#26](https://github.com/home-operations/litellm-operator/issues/26)) ([ef408c4](https://github.com/home-operations/litellm-operator/commit/ef408c478a772e26d171dea5c56deadaaca722a3))
* **llmkube:** trim embedding/rerank endpoint suffixes in api_base ([#28](https://github.com/home-operations/litellm-operator/issues/28)) ([bca6515](https://github.com/home-operations/litellm-operator/commit/bca6515ff6843e98e09ebaea3e25b7fb496ef3d5))

## [0.0.5](https://github.com/home-operations/litellm-operator/compare/0.0.4...0.0.5) (2026-07-03)


### Features

* **deps:** update module sigs.k8s.io/gateway-api (v1.5.1 → v1.6.0) ([#20](https://github.com/home-operations/litellm-operator/issues/20)) ([09c847a](https://github.com/home-operations/litellm-operator/commit/09c847a64d28e6b63af065d3c36c30d8791e6dff))
* **mcpserver:** add serviceAccount, securityContext, scheduling and pod metadata to workload ([#27](https://github.com/home-operations/litellm-operator/issues/27)) ([0024a06](https://github.com/home-operations/litellm-operator/commit/0024a06629775b9cf3bf05468a26abf67057b74d))


### Bug Fixes

* **deps:** update module github.com/defilantech/llmkube (v0.8.20 → v0.8.22) ([#17](https://github.com/home-operations/litellm-operator/issues/17)) ([66ef077](https://github.com/home-operations/litellm-operator/commit/66ef07717970886530b2f431e4030bc4c687f523))
* **deps:** update module github.com/defilantech/llmkube (v0.8.22 → v0.8.26) ([#24](https://github.com/home-operations/litellm-operator/issues/24)) ([13e6f27](https://github.com/home-operations/litellm-operator/commit/13e6f275743eb92a78558587e01e83cd2f63c904))


### Miscellaneous Chores

* **mise:** Lock file maintenance tool ([#23](https://github.com/home-operations/litellm-operator/issues/23)) ([4277d86](https://github.com/home-operations/litellm-operator/commit/4277d86961c2575333c4c2c71ce710f342184abc))
* **mise:** Update tool oxfmt (0.56.0 → 0.57.0) ([#21](https://github.com/home-operations/litellm-operator/issues/21)) ([2645f02](https://github.com/home-operations/litellm-operator/commit/2645f020f4b0ae549bc27f7c1fd92a94e32c762a))

## [0.0.4](https://github.com/home-operations/litellm-operator/compare/0.0.3...0.0.4) (2026-06-29)


### Features

* run MCP server workloads from LiteLLMMCPServer ([#19](https://github.com/home-operations/litellm-operator/issues/19)) ([2642815](https://github.com/home-operations/litellm-operator/commit/2642815457c39da5630e770f2b0a98370f1ee8c9))


### Bug Fixes

* **deps:** update module github.com/defilantech/llmkube (v0.8.19 → v0.8.20) ([#16](https://github.com/home-operations/litellm-operator/issues/16)) ([4e35963](https://github.com/home-operations/litellm-operator/commit/4e35963087de68c06b37aa653c2ad0f2165536b4))


### Miscellaneous Chores

* **renovate:** inherit shared toolchain + chart-docs presets ([#14](https://github.com/home-operations/litellm-operator/issues/14)) ([3490ae4](https://github.com/home-operations/litellm-operator/commit/3490ae44cc1d27794fbaf238cefea0d9874532a0))

## [0.0.3](https://github.com/home-operations/litellm-operator/compare/0.0.2...0.0.3) (2026-06-27)


### Features

* infer model_info.mode for LLMKube embedding and reranker services ([#11](https://github.com/home-operations/litellm-operator/issues/11)) ([caee50d](https://github.com/home-operations/litellm-operator/commit/caee50d708225b5769d365ff1576546311421e7b))


### Bug Fixes

* **deps:** update module github.com/defilantech/llmkube (v0.8.18 → v0.8.19) ([#12](https://github.com/home-operations/litellm-operator/issues/12)) ([9d01a15](https://github.com/home-operations/litellm-operator/commit/9d01a1524dd23b3512cade768352c96130f29bd5))

## [0.0.2](https://github.com/home-operations/litellm-operator/compare/0.0.1...0.0.2) (2026-06-27)


### Features

* auto-register LiteLLMModels from LLMKube InferenceServices ([1acc9dd](https://github.com/home-operations/litellm-operator/commit/1acc9dd6836763d7cfa91be9633cee061a3e8f5b))
* auto-register LiteLLMModels from LLMKube InferenceServices ([1f8866a](https://github.com/home-operations/litellm-operator/commit/1f8866ac072d2dfcdf0802065c95c55f14d459de))
* **deps:** update module github.com/onsi/ginkgo/v2 (v2.28.0 → v2.32.0) ([450cb97](https://github.com/home-operations/litellm-operator/commit/450cb976e664e1fc8ef95fda3c25585055c8f563))
* **deps:** update module github.com/onsi/ginkgo/v2 (v2.28.0 → v2.32.0) ([1938b97](https://github.com/home-operations/litellm-operator/commit/1938b971a7c48735cc3cb40d064b883aa9e6d80e))
* **deps:** update module github.com/onsi/gomega (v1.39.1 → v1.42.1) ([579d790](https://github.com/home-operations/litellm-operator/commit/579d790a17db3fab4f1b3c664892ec9624f30e21))


### Bug Fixes

* **ci:** recreate kind cluster before e2e to avoid stale node state ([f33a8fa](https://github.com/home-operations/litellm-operator/commit/f33a8fa88ba91f46a7b5f6b485843f0af9ce1ce8))


### Miscellaneous Chores

* add minimumGroupSize to Go toolchain configuration ([fd6e38b](https://github.com/home-operations/litellm-operator/commit/fd6e38bb95ccdd0063d7235100b9cc3ba8c41283))
* **mise:** Update tool jq (1.8.1 → 1.8.2) ([4714857](https://github.com/home-operations/litellm-operator/commit/47148570ee35160a3b8b12afa7f71753f654e641))
* **mise:** Update tool oxfmt (0.55.0 → 0.56.0) ([98ee03f](https://github.com/home-operations/litellm-operator/commit/98ee03f36930a58ce070a6b378e906e708276032))

## 0.0.1 (2026-06-24)


### Features

* add applyMode file|api with DB-backed model sync via a typed admin client ([a5e1ef5](https://github.com/home-operations/litellm-operator/commit/a5e1ef54f904d2d34c65b0474e3cd4eac401ae3a))
* add configurable liveness/readiness probes to managed proxy ([2893a80](https://github.com/home-operations/litellm-operator/commit/2893a809c1f83fff00f4bbffe7c464238b7ff48b))
* add extraConfig passthrough for arbitrary top-level config keys ([2021f95](https://github.com/home-operations/litellm-operator/commit/2021f955f3384576b527743b03a4701a5e1df9db))
* add LiteLLMGuardrail and LiteLLMMCPServer CRDs, typed callbacks, and named top-level config blocks ([400facb](https://github.com/home-operations/litellm-operator/commit/400facb37a7504d8104476ed516b623caa2ff838))
* add validating webhook, e2e tests, and CI parity with org template ([57076c9](https://github.com/home-operations/litellm-operator/commit/57076c90d470f4597ba86308bf6a481ca3b92b6d))
* initial litellm-operator ([9ba1b2d](https://github.com/home-operations/litellm-operator/commit/9ba1b2d8c72dd1a8dd3a7cc0968ad51b01e40e7c))
* typed model info, secret refs, namespace binding, and HTTPRoute support ([ebbc6d2](https://github.com/home-operations/litellm-operator/commit/ebbc6d2836ec2a583ed609220e7ef16520075daf))


### Bug Fixes

* **ci:** exclude e2e package from the unit test task ([e60d7e0](https://github.com/home-operations/litellm-operator/commit/e60d7e07b7143ee19fe5109e39937dbb345fa7e9))
* **release:** set initial-version to 0.0.1 so the first release is not 1.0.0 ([5e2ef65](https://github.com/home-operations/litellm-operator/commit/5e2ef65daabf7f2580b96c51f04e5771f1f208f1))


### Miscellaneous Chores

* **release:** bootstrap versioning so the first release is 0.0.1 ([ed175a0](https://github.com/home-operations/litellm-operator/commit/ed175a0835290f40d6f86fb675fd52ef6dd2b124))


### Code Refactoring

* backstop env-var collision in renderer and simplify deployment apply ([4b726a6](https://github.com/home-operations/litellm-operator/commit/4b726a600ca6a90bc91b276c6e49211ef5497f32))

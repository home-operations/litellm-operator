# Changelog

## [0.0.16](https://github.com/home-operations/litellm-operator/compare/0.0.15...0.0.16) (2026-08-22)


### Features

* **go:** update module github.com/stretchr/testify (v1.11.1 → v1.12.1) ([#107](https://github.com/home-operations/litellm-operator/issues/107)) ([f9e5a68](https://github.com/home-operations/litellm-operator/commit/f9e5a686f47e490ba8d5db821c645c2067179b63))


### Bug Fixes

* **go:** update kubernetes monorepo (v0.36.3 → v0.36.4) ([#110](https://github.com/home-operations/litellm-operator/issues/110)) ([03b55d0](https://github.com/home-operations/litellm-operator/commit/03b55d0d3c8b586b475eed7831fba6b263f3939d))
* **go:** update module github.com/defilantech/llmkube (v0.9.16 → v0.9.19) ([#100](https://github.com/home-operations/litellm-operator/issues/100)) ([6741794](https://github.com/home-operations/litellm-operator/commit/6741794dc6025d1af28fe81a976fa260d751ea51))


### Miscellaneous Chores

* **github-action:** update action jdx/mise-action (v4.2.4 → v4.2.5) ([#103](https://github.com/home-operations/litellm-operator/issues/103)) ([6aa0704](https://github.com/home-operations/litellm-operator/commit/6aa07047dbff7e46effcedb85f5877341ee04a23))
* **mise:** update mise tools ([#102](https://github.com/home-operations/litellm-operator/issues/102)) ([1141a7c](https://github.com/home-operations/litellm-operator/commit/1141a7caacc6761df58038a50c8fab10590fc138))
* **mise:** update tool oxfmt (0.62.0 → 0.63.0) ([#99](https://github.com/home-operations/litellm-operator/issues/99)) ([c782e21](https://github.com/home-operations/litellm-operator/commit/c782e21be0efc804caddd7e965f21bdd691dd5ef))
* **mise:** update tool oxfmt (0.63.0 → 0.64.0) ([#114](https://github.com/home-operations/litellm-operator/issues/114)) ([e4b2d78](https://github.com/home-operations/litellm-operator/commit/e4b2d781d29cbd92d14a57ee49fe1a1510c89cce))

## [0.0.15](https://github.com/home-operations/litellm-operator/compare/0.0.14...0.0.15) (2026-08-21)


### Bug Fixes

* watch generated HTTPRoutes ([7bbc721](https://github.com/home-operations/litellm-operator/commit/7bbc72185de99b2be009298e444ccee29778969a))


### Tests

* install Gateway API CRDs for e2e ([15c382f](https://github.com/home-operations/litellm-operator/commit/15c382f5ad67cc6671411b8cc349ffcf7b90d350))

## [0.0.14](https://github.com/home-operations/litellm-operator/compare/0.0.13...0.0.14) (2026-08-20)


### Features

* support HTTPRoute filters ([743a737](https://github.com/home-operations/litellm-operator/commit/743a737c565dd6c39019e8ead6fdd1be9ce75c2b))


### Bug Fixes

* **go:** update module github.com/defilantech/llmkube (v0.9.13 → v0.9.14) ([#89](https://github.com/home-operations/litellm-operator/issues/89)) ([71439e9](https://github.com/home-operations/litellm-operator/commit/71439e95a9e64a79ca345bf917d9c7be4839c31f))
* **go:** update module github.com/defilantech/llmkube (v0.9.14 → v0.9.16) ([#96](https://github.com/home-operations/litellm-operator/issues/96)) ([b5889e1](https://github.com/home-operations/litellm-operator/commit/b5889e1840772e3a55d70b9580c0acd540dd2f2d))
* keep HTTPRoute filters CRD validation valid ([1804f53](https://github.com/home-operations/litellm-operator/commit/1804f535bc3bbd78a1b6a779620d604f3edf9258))
* reconcile virtual key spec changes ([750d7ab](https://github.com/home-operations/litellm-operator/commit/750d7ab5c6a6103cd95046f07a1deeb95f132bc0))


### Documentation

* describe the pinned go directive policy ([#104](https://github.com/home-operations/litellm-operator/issues/104)) ([91f8448](https://github.com/home-operations/litellm-operator/commit/91f844886621d8b3fd2cc6728d3055229534f359))


### Continuous Integration

* **github-action:** Update action docker/github-builder (v1.15.0 → v1.16.0) ([#94](https://github.com/home-operations/litellm-operator/issues/94)) ([5bd2e07](https://github.com/home-operations/litellm-operator/commit/5bd2e07f87d9891155773aee7eb145ae6debc648))


### Miscellaneous Chores

* **mise:** Update tool cosign (3.1.2 → 3.1.3) ([#97](https://github.com/home-operations/litellm-operator/issues/97)) ([dd85bfc](https://github.com/home-operations/litellm-operator/commit/dd85bfcf3cb21473befc2d43d4c25e3e5931a02d))
* **mise:** update tool go (1.26.5 → 1.26.6) ([#106](https://github.com/home-operations/litellm-operator/issues/106)) ([b510085](https://github.com/home-operations/litellm-operator/commit/b5100852026c288efdb8faa5b3cc3f452a7b5a21))

## [0.0.13](https://github.com/home-operations/litellm-operator/compare/0.0.12...0.0.13) (2026-08-07)


### Miscellaneous Chores

* **mise:** Update tool oxfmt (0.61.0 → 0.62.0) ([#91](https://github.com/home-operations/litellm-operator/issues/91)) ([7fdc392](https://github.com/home-operations/litellm-operator/commit/7fdc392758230d82ccd91233f94b481ae836b8fa))

## [0.0.12](https://github.com/home-operations/litellm-operator/compare/0.0.11...0.0.12) (2026-08-06)


### Bug Fixes

* **chart:** grant RBAC for LiteLLM teams and virtual keys ([d3034d4](https://github.com/home-operations/litellm-operator/commit/d3034d4b06ac400c3e2f64bb1e7c807739c5ef4c))

## [0.0.11](https://github.com/home-operations/litellm-operator/compare/0.0.10...0.0.11) (2026-08-05)


### Continuous Integration

* **github-action:** Update action jdx/mise-action (v4.2.3 → v4.2.4) ([#88](https://github.com/home-operations/litellm-operator/issues/88)) ([d3e3d29](https://github.com/home-operations/litellm-operator/commit/d3e3d299039513cb2113420922e6a7ea68a0504b))

## [0.0.10](https://github.com/home-operations/litellm-operator/compare/0.0.9...0.0.10) (2026-08-03)


### Bug Fixes

* **ci:** fail the merge gate on cancelled jobs, and key the lint cache on the toolchain ([#77](https://github.com/home-operations/litellm-operator/issues/77)) ([43c67ab](https://github.com/home-operations/litellm-operator/commit/43c67abba0d827dc41c057d97a410f82850ce83d))
* **deps:** update module github.com/defilantech/llmkube (v0.9.5 → v0.9.6) ([#45](https://github.com/home-operations/litellm-operator/issues/45)) ([05ae88c](https://github.com/home-operations/litellm-operator/commit/05ae88cfb0b74eca6100ff2f905be71f99d643ee))
* **deps:** update module github.com/defilantech/llmkube (v0.9.6 → v0.9.8) ([#47](https://github.com/home-operations/litellm-operator/issues/47)) ([2d5fe66](https://github.com/home-operations/litellm-operator/commit/2d5fe660aadc533e4075d5c4cdbdb63502806159))
* **deps:** update module sigs.k8s.io/gateway-api (v1.6.0 → v1.6.1) ([#46](https://github.com/home-operations/litellm-operator/issues/46)) ([f3bb7f9](https://github.com/home-operations/litellm-operator/commit/f3bb7f942c7689bc1c155b1c6c8c2087ca0f973d))
* **e2e:** read one-shot pod logs instead of kubectl run --attach ([#64](https://github.com/home-operations/litellm-operator/issues/64)) ([a0a3c87](https://github.com/home-operations/litellm-operator/commit/a0a3c877696196f27439a9075a042b66109ce8bd))
* **go:** update kubernetes monorepo (v0.36.2 → v0.36.3) ([#56](https://github.com/home-operations/litellm-operator/issues/56)) ([f4347bc](https://github.com/home-operations/litellm-operator/commit/f4347bcaf3337af769f5d98923decd875c83eeb7))
* **go:** update module github.com/defilantech/llmkube (v0.9.11 → v0.9.12) ([#67](https://github.com/home-operations/litellm-operator/issues/67)) ([34e440f](https://github.com/home-operations/litellm-operator/commit/34e440f7c42180f53d07fc7d0641188a6ad90c2c))
* **go:** update module github.com/defilantech/llmkube (v0.9.12 → v0.9.13) ([#75](https://github.com/home-operations/litellm-operator/issues/75)) ([a6ecca7](https://github.com/home-operations/litellm-operator/commit/a6ecca78b9e76be7eedb0c509462ccedad06ce66))
* **go:** update module github.com/defilantech/llmkube (v0.9.8 → v0.9.11) ([#53](https://github.com/home-operations/litellm-operator/issues/53)) ([75d4966](https://github.com/home-operations/litellm-operator/commit/75d4966d2a8e9ca1f79c41b0aab5502bfb153f07))
* **helm:** stamp Chart.yaml version on release ([#60](https://github.com/home-operations/litellm-operator/issues/60)) ([e4e74d7](https://github.com/home-operations/litellm-operator/commit/e4e74d77d398659ce2c3d7fd1fabea71a80ceceb))
* **lint:** point .prettierignore at this repo's generated files ([#61](https://github.com/home-operations/litellm-operator/issues/61)) ([aff6725](https://github.com/home-operations/litellm-operator/commit/aff67253ea81d7df63a26541172b098d5ad84a57))


### Documentation

* add AGENTS.md with Go conventions ([#81](https://github.com/home-operations/litellm-operator/issues/81)) ([a1df609](https://github.com/home-operations/litellm-operator/commit/a1df6095ca26d8e5d67c108e9dc819f4a985b44c))


### Styles

* indent markdown at 2 to match embedded yaml ([#50](https://github.com/home-operations/litellm-operator/issues/50)) ([ae5f0db](https://github.com/home-operations/litellm-operator/commit/ae5f0db31a25696cba92889848fa19f8c4f43c6a))


### Build System

* **mise:** add actionlint and refresh the lockfile ([#65](https://github.com/home-operations/litellm-operator/issues/65)) ([c69c856](https://github.com/home-operations/litellm-operator/commit/c69c85605daedca580a7491305f94a1abce77622))


### Continuous Integration

* gate pull requests on a single Build Success check ([#63](https://github.com/home-operations/litellm-operator/issues/63)) ([ed34d01](https://github.com/home-operations/litellm-operator/commit/ed34d01ad41e3dbc23d27dbffd206b3b2a7ac749))
* **github-action:** Update action actions/checkout (v7.0.0 → v7.0.1) ([d5966b7](https://github.com/home-operations/litellm-operator/commit/d5966b7efc0a64c7a9207e304471e9603d9d8212))
* **github-action:** Update action actions/stale (v10.4.0 → v11.0.0) ([#78](https://github.com/home-operations/litellm-operator/issues/78)) ([2fcdc92](https://github.com/home-operations/litellm-operator/commit/2fcdc92b50eb9dce04d63864ddd92dd973615416))
* **github-action:** Update action docker/github-builder (v1.13.0 → v1.14.0) ([8de0482](https://github.com/home-operations/litellm-operator/commit/8de0482df44c119dbfc06dc0bccb0e60ed37078d))
* **github-action:** Update action docker/github-builder (v1.14.0 → v1.15.0) ([#76](https://github.com/home-operations/litellm-operator/issues/76)) ([4dd3b61](https://github.com/home-operations/litellm-operator/commit/4dd3b61aff09ae15246027b4b1e7ede6e30b40da))
* **github-action:** Update action docker/login-action (v4.4.0 → v4.5.0) ([768fd15](https://github.com/home-operations/litellm-operator/commit/768fd15df69f723a1eac77820e7b79d992ed3556))
* **github-action:** Update action docker/login-action (v4.5.0 → v4.5.1) ([#69](https://github.com/home-operations/litellm-operator/issues/69)) ([755b002](https://github.com/home-operations/litellm-operator/commit/755b002f093c7eb2d24b02eb7cb1b1d7782367b0))
* **github-action:** Update action docker/login-action (v4.5.1 → v4.5.2) ([#79](https://github.com/home-operations/litellm-operator/issues/79)) ([85c596e](https://github.com/home-operations/litellm-operator/commit/85c596ea31776e7edff75b6c839ae5c964c0b4cb))
* **github-action:** Update action docker/login-action (v4.5.2 → v4.6.0) ([#82](https://github.com/home-operations/litellm-operator/issues/82)) ([652e082](https://github.com/home-operations/litellm-operator/commit/652e082ff0b6449f8882b1442c0b5f057f1f31ec))
* **github-action:** Update action home-operations/.github/actions/workflow-lint (v1.0.2 → v1.0.3) ([#87](https://github.com/home-operations/litellm-operator/issues/87)) ([0a35eac](https://github.com/home-operations/litellm-operator/commit/0a35eac8bc2cd1860d21c1bcf9bd2da1945eb25a))
* **github-action:** Update action jdx/mise-action (v4.2.0 → v4.2.1) ([05e9637](https://github.com/home-operations/litellm-operator/commit/05e963729fbcbf1407978810c4366467063e0180))
* **github-action:** Update action jdx/mise-action (v4.2.1 → v4.2.2) ([#68](https://github.com/home-operations/litellm-operator/issues/68)) ([af5f877](https://github.com/home-operations/litellm-operator/commit/af5f8774e86ebda6e08fb611fa877bdf244f07ea))
* **github-action:** Update action jdx/mise-action (v4.2.2 → v4.2.3) ([#71](https://github.com/home-operations/litellm-operator/issues/71)) ([0f5a326](https://github.com/home-operations/litellm-operator/commit/0f5a3268720093ce79df95150fdfa8d08ec925f3))
* **github-action:** update workflow-lint action (1.0.0 → v1.0.2) ([#84](https://github.com/home-operations/litellm-operator/issues/84)) ([9135bd8](https://github.com/home-operations/litellm-operator/commit/9135bd8f726eeabd38705f849e15902aa2ec6f78))
* lint workflows with the shared composite action ([#66](https://github.com/home-operations/litellm-operator/issues/66)) ([db40684](https://github.com/home-operations/litellm-operator/commit/db40684efc3cc38f3f5ffc61d2a2f5850fefa8c5))
* **renovate:** reactive dashboard + config runs in one workflow ([#58](https://github.com/home-operations/litellm-operator/issues/58)) ([607a9cb](https://github.com/home-operations/litellm-operator/commit/607a9cb99f64b99a7bbfea48d6a8380bbb9f23bf))
* skip release-please churn and stop cancelling main runs ([#48](https://github.com/home-operations/litellm-operator/issues/48)) ([72a65fb](https://github.com/home-operations/litellm-operator/commit/72a65fb5f652b2ee13922795949525f89a9000cb))
* skip release-please version-bump PRs in checks ([#62](https://github.com/home-operations/litellm-operator/issues/62)) ([03a14cf](https://github.com/home-operations/litellm-operator/commit/03a14cfe91bb830c388f70436e975d135f266152))
* wire govulncheck into mise and CI ([#86](https://github.com/home-operations/litellm-operator/issues/86)) ([2959040](https://github.com/home-operations/litellm-operator/commit/2959040c882366e83990fd3e47e0018f433c356c))


### Miscellaneous Chores

* add zizmor ([e3d0934](https://github.com/home-operations/litellm-operator/commit/e3d0934eb8893d07fbb499184b5a074154efdd31))
* **github-release:** Update release helm-unittest/helm-unittest (v1.1.1 → v1.1.2) ([#59](https://github.com/home-operations/litellm-operator/issues/59)) ([5bac541](https://github.com/home-operations/litellm-operator/commit/5bac54130833132a4ad50a64fff08027bc15dad4))
* **mise:** Lock file maintenance tool ([#54](https://github.com/home-operations/litellm-operator/issues/54)) ([b6bdda7](https://github.com/home-operations/litellm-operator/commit/b6bdda700d23db806d35eaf46f351e7cc717da0e))
* **mise:** prune lockfile to used platforms ([#85](https://github.com/home-operations/litellm-operator/issues/85)) ([e3838d7](https://github.com/home-operations/litellm-operator/commit/e3838d7b1481b5f9abb8e5413e2ff6bc404e394b))
* **mise:** Update tool cosign (3.1.1 → 3.1.2) ([#49](https://github.com/home-operations/litellm-operator/issues/49)) ([71d1dbc](https://github.com/home-operations/litellm-operator/commit/71d1dbcdbec57046eee527154f35fd22f478d4b6))
* **mise:** Update tool kubectl (1.36.2 → 1.36.3) ([#57](https://github.com/home-operations/litellm-operator/issues/57)) ([23c20d6](https://github.com/home-operations/litellm-operator/commit/23c20d651ee595f8d20fd2a141706175b0380f6a))
* **mise:** Update tool oxfmt (0.58.0 → 0.59.0) ([#43](https://github.com/home-operations/litellm-operator/issues/43)) ([2bef1ee](https://github.com/home-operations/litellm-operator/commit/2bef1ee5b190f6e9b41b1d107b3df8db86fd1653))
* **mise:** Update tool oxfmt (0.59.0 → 0.60.0) ([#52](https://github.com/home-operations/litellm-operator/issues/52)) ([5da78ec](https://github.com/home-operations/litellm-operator/commit/5da78ecf41f015386044a95dc7bebcf4c3127b65))
* **mise:** Update tool oxfmt (0.60.0 → 0.61.0) ([#70](https://github.com/home-operations/litellm-operator/issues/70)) ([d287e41](https://github.com/home-operations/litellm-operator/commit/d287e4114b52cfbc3177ad51bea3af4d078dd67e))
* **mise:** Update tool zizmor (1.27.0 → 1.28.0) ([#51](https://github.com/home-operations/litellm-operator/issues/51)) ([cd171d6](https://github.com/home-operations/litellm-operator/commit/cd171d62874e0a7794abe42944dc38a4dc72851a))
* **mise:** Update tool zizmor (1.28.0 → 1.29.0) ([#83](https://github.com/home-operations/litellm-operator/issues/83)) ([82e2b1e](https://github.com/home-operations/litellm-operator/commit/82e2b1e8f326a2265f6d511214d133d8a6114500))
* **release-please:** standardize the release pull request title pattern ([#80](https://github.com/home-operations/litellm-operator/issues/80)) ([49a33c5](https://github.com/home-operations/litellm-operator/commit/49a33c5089f3d3c691c0215da3d7a51dce3eca8f))
* standardize release-please changelog sections ([#74](https://github.com/home-operations/litellm-operator/issues/74)) ([b4f8cea](https://github.com/home-operations/litellm-operator/commit/b4f8cea9a9b791d5397f7171c3ab64088b01499c))

## [0.0.9](https://github.com/home-operations/litellm-operator/compare/0.0.8...0.0.9) (2026-07-13)


### Bug Fixes

* **deps:** update module github.com/defilantech/llmkube (v0.9.1 → v0.9.5) ([#38](https://github.com/home-operations/litellm-operator/issues/38)) ([5457d65](https://github.com/home-operations/litellm-operator/commit/5457d657d98551c0b543decd2eae19c6415603f5))

## [0.0.8](https://github.com/home-operations/litellm-operator/compare/0.0.7...0.0.8) (2026-07-10)


### Features

* **deps:** update module github.com/defilantech/llmkube (v0.8.28 → v0.9.0) ([#32](https://github.com/home-operations/litellm-operator/issues/32)) ([8f98fef](https://github.com/home-operations/litellm-operator/commit/8f98fef484f1eda2834449e2948ca1c41b4c1ad3))
* **proxy:** add volumes, volumeMounts and pod metadata to workload ([#41](https://github.com/home-operations/litellm-operator/issues/41)) ([dbbf078](https://github.com/home-operations/litellm-operator/commit/dbbf078cc8a42479a53e3e4b52dd880414cf37ec))


### Bug Fixes

* **deps:** update module github.com/defilantech/llmkube (v0.9.0 → v0.9.1) ([#35](https://github.com/home-operations/litellm-operator/issues/35)) ([2695919](https://github.com/home-operations/litellm-operator/commit/2695919b0b28def7438e2d26259766bf062ee469))


### Miscellaneous Chores

* **mise:** Update tool go (1.26.4 → 1.26.5) ([#37](https://github.com/home-operations/litellm-operator/issues/37)) ([46112b8](https://github.com/home-operations/litellm-operator/commit/46112b8b4e92c3faf8041bca5f37c9c3189906d9))
* **mise:** Update tool helm (4.2.2 → 4.2.3) ([#39](https://github.com/home-operations/litellm-operator/issues/39)) ([a6cad6f](https://github.com/home-operations/litellm-operator/commit/a6cad6faf2dd4b5cffd8b0a008fe33e49884cac3))
* **mise:** Update tool lefthook (2.1.9 → 2.1.10) ([#36](https://github.com/home-operations/litellm-operator/issues/36)) ([4e81587](https://github.com/home-operations/litellm-operator/commit/4e8158710ea7bf54336702a6085f80dbda1d0c52))
* **mise:** Update tool oxfmt (0.57.0 → 0.58.0) ([#34](https://github.com/home-operations/litellm-operator/issues/34)) ([4d2e7e4](https://github.com/home-operations/litellm-operator/commit/4d2e7e4914ae949e037c6b5809c8c19e03eaf438))

## [0.0.7](https://github.com/home-operations/litellm-operator/compare/0.0.6...0.0.7) (2026-07-04)


### Features

* consolidate on a single plain-HTTP operational port (org standard) ([#30](https://github.com/home-operations/litellm-operator/issues/30)) ([b36cd31](https://github.com/home-operations/litellm-operator/commit/b36cd311fe2cecc1a4cc05a2a9857cc6b754d65c))

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

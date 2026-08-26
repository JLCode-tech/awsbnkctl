# Changelog

All notable changes to `awsbnkctl` are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project uses [semantic versioning](https://semver.org/spec/v2.0.0.html). Pre-`v1.0.0` minor versions may include breaking changes — see the per-version notes.

## 1.0.0 (2026-08-26)


### Features

* **agentcore-demo:** a real stranger for path 3, and prove the firewall ([6088c47](https://github.com/JLCode-tech/awsbnkctl/commit/6088c4727f50a33c3f459ef266641bf0d4f3359f))
* **agentcore-demo:** BNK governance + end-to-end token observability ([7b23469](https://github.com/JLCode-tech/awsbnkctl/commit/7b234699dd44597348da9f709841fa839bb2e474))
* **agentcore-demo:** guided demo driver and a slide diagram ([65c2300](https://github.com/JLCode-tech/awsbnkctl/commit/65c2300b1574f32fe753e8e0cc4524cf59b5b3e6))
* **agentcore-demo:** one-command rebuild, and fix a stale-kubeconfig trap ([235fa09](https://github.com/JLCode-tech/awsbnkctl/commit/235fa099b43d19f0ed77f33a5a10dd8466500f7b))
* **agentcore-demo:** real authn/authz, a privileged tool, and honest telemetry ([1089364](https://github.com/JLCode-tech/awsbnkctl/commit/10893644a5be6034a56586c3d7609a2f1682568f))
* **agentcore-demo:** TLS on the MCP hop — and retract the "BNK can't do HTTPS" claim ([e9d6534](https://github.com/JLCode-tech/awsbnkctl/commit/e9d6534bb7bf5b4faf7e82fd7cedbfad950ac307))
* **ai-rig:** enable AWS LB Controller addon for Envoy proxy shootout (PRD-11) ([e38c1bd](https://github.com/JLCode-tech/awsbnkctl/commit/e38c1bd68451a728d37477707a8e6385175953ed))
* **benchmark:** real Envoy-vs-BNK shootout via NodePort pre-seed (WS-E2) ([3d5acf2](https://github.com/JLCode-tech/awsbnkctl/commit/3d5acf2117a7e1bf5030d30c4589970783d487d8))
* **benchmark:** SageMaker cold-redeploy cache reset behind --reset-cache (PRD-11 Task B) ([6214e70](https://github.com/JLCode-tech/awsbnkctl/commit/6214e701e21f3e06a0f4c07eb4744f51781ac487))
* **benchmark:** scenario-sequence + cache-reset seam (PRD-11 Task A) ([d50f09d](https://github.com/JLCode-tech/awsbnkctl/commit/d50f09d44040f766fcaf2595765bb1a2525553a2))
* **benchmark:** set opt-in internal-NLB tags on forge target for non-BNK shootouts (PRD-11 Slice 4) ([16fa278](https://github.com/JLCode-tech/awsbnkctl/commit/16fa2785c4dafbe799a775fdf595ad58eba35822))
* **bnk:** resync --watch — auto-heal stale pool members on EndpointSlice change ([779d75a](https://github.com/JLCode-tech/awsbnkctl/commit/779d75aa033ab9208af7c7a0e19600afe45937d7))
* **cli:** add 'awsbnkctl topology' — whole-cluster data-path view (ASCII + mermaid) ([76b9112](https://github.com/JLCode-tech/awsbnkctl/commit/76b91126f07b4759829c059325d7b79eaff05017))
* **cli:** add `up --demo` flag + `demo:` config block + DEMO_MODE state marker ([#68](https://github.com/JLCode-tech/awsbnkctl/issues/68)) ([3942d1f](https://github.com/JLCode-tech/awsbnkctl/commit/3942d1f669c9161e3cf1af9e48a9ff94d91d8180))
* **cli:** per-phase deployment lines in \`awsbnkctl status\` ([8bfd8ab](https://github.com/JLCode-tech/awsbnkctl/commit/8bfd8abf726060e29b0a905f225c7938737b1ba1))
* **cni:** enable VPC CNI prefix delegation to keep pods on the primary ENI ([f5d2bda](https://github.com/JLCode-tech/awsbnkctl/commit/f5d2bdab2f590a3f3a7b58f92fa02b1bdb640496))
* complete agentcore demo with gateway api routing ([295f016](https://github.com/JLCode-tech/awsbnkctl/commit/295f0169f5a6a767ef2805baa4da9a62340ac9a1))
* complete BNK infrastructure routing and test clients for A2A and MCP governance ([006b650](https://github.com/JLCode-tech/awsbnkctl/commit/006b6502ba2eaa8f741a97f029998224cb963650))
* **demo:** ingress-migration + external BIG-IP VE/CIS demo scenarios ([9fb69bb](https://github.com/JLCode-tech/awsbnkctl/commit/9fb69bbe0a77b69abd5dcce5bb16c85715b77ae5))
* **demo:** promote http2+diameter assets into //go:embed packages (PRD-10 Embed slice) ([#71](https://github.com/JLCode-tech/awsbnkctl/issues/71)) ([9fb8c4f](https://github.com/JLCode-tech/awsbnkctl/commit/9fb8c4fcd58a26740735c612a58ed68ee4b65d66))
* **demo:** Slice A2 — inject awsbnkctl:demo + demo-expiry tags onto all AWS resources ([#69](https://github.com/JLCode-tech/awsbnkctl/issues/69)) ([987a36b](https://github.com/JLCode-tech/awsbnkctl/commit/987a36be7652a78b271fce2949af137de4377b6c))
* **demo:** Slice B — Phase17dDemoStage jumphost client pre-staging ([#73](https://github.com/JLCode-tech/awsbnkctl/issues/73)) ([bbbd601](https://github.com/JLCode-tech/awsbnkctl/commit/bbbd601eec9e1efcf8b7f28923443876883a7973))
* **demo:** Slice C0 — demo {list,run,clean} command group + clean enumerator ([#75](https://github.com/JLCode-tech/awsbnkctl/issues/75)) ([c859267](https://github.com/JLCode-tech/awsbnkctl/commit/c859267874f4907e9de9f653324c310f2938cfab))
* **demo:** Slice C1 — http2 use-case (HTTP/2 h2c end-to-end through TMM) (PRD-10) ([#76](https://github.com/JLCode-tech/awsbnkctl/issues/76)) ([9d0fa97](https://github.com/JLCode-tech/awsbnkctl/commit/9d0fa97d27a0e27de053bad91e5b2c7ee4bf16ff))
* **demo:** Slice C2 — diameter use-case (L4 CER→CEA via BNK TMM) (PRD-10) ([#77](https://github.com/JLCode-tech/awsbnkctl/issues/77)) ([9739318](https://github.com/JLCode-tech/awsbnkctl/commit/973931873248f79194bbeeeed81eae68b20716cb))
* **demo:** Slice D — green scenarios in the demo catalogue (PRD-10) ([#78](https://github.com/JLCode-tech/awsbnkctl/issues/78)) ([cc88256](https://github.com/JLCode-tech/awsbnkctl/commit/cc8825697a5d8509752ac923a2dcae5e96fe84c1))
* **forge:** --register-with-forge flag on awsbnkctl up (P2) ([6119a21](https://github.com/JLCode-tech/awsbnkctl/commit/6119a21c92c6a3034361bfb3ebbe098c66a79457))
* **forge-benchmark:** mooncake trace replay + registry-derived scenario help (WS-C2) ([31c48e9](https://github.com/JLCode-tech/awsbnkctl/commit/31c48e98cfbca760a797e616d55f33aeddc828f0))
* **forge-benchmark:** port forge native scenario engine (WS-C1) ([90c3dcb](https://github.com/JLCode-tech/awsbnkctl/commit/90c3dcbbf58269407eccf379b1188b990138296b))
* **forge-benchmark:** rich raw /aiperf push + object-graph linkage + aiperf install hardening ([293ac3b](https://github.com/JLCode-tech/awsbnkctl/commit/293ac3b96029205cfaaedc32ad13510c4832d2a7))
* forge-enabled demo example + live rocket timer ([#89](https://github.com/JLCode-tech/awsbnkctl/issues/89)) ([857ae2f](https://github.com/JLCode-tech/awsbnkctl/commit/857ae2fb7f7cc8e3372706f66db2a2d93c05cec9))
* **forge:** delete benchmark artifacts on down + opt-in forge cleanup ([42e24c0](https://github.com/JLCode-tech/awsbnkctl/commit/42e24c0d7efeb8819595a9893d1d37a0941f932e))
* **forge:** make forge credentials + REST URL configurable (flag/env/yaml) ([537879e](https://github.com/JLCode-tech/awsbnkctl/commit/537879e08316edf20ba701d14bde079c10b98a4b))
* **forge:** multi-scenario benchmark presets + jumphost access-method registration ([7bd4baa](https://github.com/JLCode-tech/awsbnkctl/commit/7bd4baa1cbde7756e735a48320325e56af3e3305))
* **forge:** proxy shootout — run one scenario through each front-end, stamp proxy_deployment_id (WS-D1) ([1735462](https://github.com/JLCode-tech/awsbnkctl/commit/1735462eb4134df364d4f5ad94a4c2ccf9e05cd5))
* **forge:** wire awsbnkctl to bnk-forge over MCP transport ([38a1081](https://github.com/JLCode-tech/awsbnkctl/commit/38a1081cb161d065ff1d8c50be57a866197fa8d5))
* **gpu:** Group 4 — Phase11cNvidiaDevicePlugin up/down + lifecycle wiring + render golden tests ([0a07f6b](https://github.com/JLCode-tech/awsbnkctl/commit/0a07f6b961bb090d76a70eeb57e407825642fed9))
* **gpu:** Group 5 — examples/ai-rig/cluster.yaml + example load tests ([e8d747c](https://github.com/JLCode-tech/awsbnkctl/commit/e8d747ccaf4d5d0d326f82bd511cf9222e203f54))
* **gpu:** Groups 1-3 — intent fields + phase10 per-ng AMI/spot/taints/AZ + NVIDIA manifest/render ([40474e3](https://github.com/JLCode-tech/awsbnkctl/commit/40474e3989b8d60f2b33da84411cdb7c7aeee59b))
* **host-device:** pciBusID NADs + node-side MAC discovery of iface/PCI ([7895000](https://github.com/JLCode-tech/awsbnkctl/commit/78950003487031f4b8ecdb690a2907e7d2c62f2c))
* **intent:** mandate Kubernetes 1.32 as the floor for real clusters ([4ae79ea](https://github.com/JLCode-tech/awsbnkctl/commit/4ae79eab1efac3ee5ab30a28a0e7ab41bb95d1f2))
* **lb-controller:** AWS Load Balancer Controller addon for internal-NLB proxy reachability (PRD-11 Slice 1) ([a94e463](https://github.com/JLCode-tech/awsbnkctl/commit/a94e463c71aedc5ae74eadfeac99ec38600d7e20))
* **network:** sriov-external pattern — TMM DPDK dataplane over vfio-bound ENA ([#92](https://github.com/JLCode-tech/awsbnkctl/issues/92)) ([7eae12c](https://github.com/JLCode-tech/awsbnkctl/commit/7eae12c3921e8b23f82fe808bc9c2d5157b7b293))
* **network:** three selectable BNK interface patterns (A/B/C) ([#91](https://github.com/JLCode-tech/awsbnkctl/issues/91)) ([8d5c359](https://github.com/JLCode-tech/awsbnkctl/commit/8d5c35901d32c5292bde63e1511c317ac049a409))
* **phase24c:** H4 — heal pod-manager cold-start race vs kube-proxy ([199675e](https://github.com/JLCode-tech/awsbnkctl/commit/199675e08691faff8eeaf1d71a0b797a1aa6682e))
* **prd-11:** AI LB + SageMaker demo slices + live-run fixes (checkpoint) ([8abcc23](https://github.com/JLCode-tech/awsbnkctl/commit/8abcc2388803e2042ea8b0f0c9468c14a075e787))
* real agentcore project via CLI instead of dummy yaml ([3ecf51f](https://github.com/JLCode-tech/awsbnkctl/commit/3ecf51f939510e3e426e09b86a6137c0df985887))
* replace dummy mcp with real fastmcp server and add agentcore agent code ([e09f64d](https://github.com/JLCode-tech/awsbnkctl/commit/e09f64df54e88d08e8f6098ccdea584e12c843e6))
* **sagemaker:** Qwen-32B sizing knobs + GPU-fit preflight (PRD-11 Task C) ([9d20ad2](https://github.com/JLCode-tech/awsbnkctl/commit/9d20ad29492dff638a862e10dcb23053a0422d16))
* **scenarios:** add AI how-tos [#6](https://github.com/JLCode-tech/awsbnkctl/issues/6)/[#7](https://github.com/JLCode-tech/awsbnkctl/issues/7) as Amber (annotation-driven) ([0a65581](https://github.com/JLCode-tech/awsbnkctl/commit/0a655817fccdc5460a0fd4898331fd73b7dfb38a))
* **scenarios:** add egress-snat Amber scenario (F5SPKEgress AUTOMAP + pseudo-CNI VXLAN) ([8edd8ed](https://github.com/JLCode-tech/awsbnkctl/commit/8edd8ede7dd9211dcf442bbbbbadd3309aad4232))
* **scenarios:** add external-resource-pool (how-to [#10](https://github.com/JLCode-tech/awsbnkctl/issues/10), Pool CR backend) ([72a23b8](https://github.com/JLCode-tech/awsbnkctl/commit/72a23b8387b69759d01ffc3a61c05538341bfd9a))
* **scenarios:** add http-traffic-split (weighted HTTPRoute, how-to [#8](https://github.com/JLCode-tech/awsbnkctl/issues/8)) ([46d84c0](https://github.com/JLCode-tech/awsbnkctl/commit/46d84c0caec1ce772f6f7d4c90642eb37dfff0fe))
* **scenarios:** add multi-vip (chassis model — N VIPs from one pool) ([b0f5550](https://github.com/JLCode-tech/awsbnkctl/commit/b0f5550cd74a132cc7aeef14b3012c75c527bf8c))
* **scenarios:** add proxy-protocol-l4 (how-to [#9](https://github.com/JLCode-tech/awsbnkctl/issues/9), L4Route + iRule) ([a856833](https://github.com/JLCode-tech/awsbnkctl/commit/a85683348d259553685b358fefed248363c1d756))
* **slice-01:** tracer-bullet VPC + subnets via strict Go SDK ([8b1ddd4](https://github.com/JLCode-tech/awsbnkctl/commit/8b1ddd4a57bdac9af34aad9f21b03a1e36a131fc))
* **slice-02:** IAM phase — EKS cluster role + node role + instance profile ([5e8d0c6](https://github.com/JLCode-tech/awsbnkctl/commit/5e8d0c6e203b662e9b625d8538391c7bb7d59e6b))
* **slice-03+04:** EKS cluster + node group + kubeconfig + forge register ([7131c5c](https://github.com/JLCode-tech/awsbnkctl/commit/7131c5ce39f733f70f2488388c605e05987b917d))
* **slice-05:** BNK foundation — namespaces + supply-chain Secrets + cert-manager + cert chain ([fd3ab15](https://github.com/JLCode-tech/awsbnkctl/commit/fd3ab15d595ea943e7ecc844f2284fc5e57ed8c7))
* **slice-06:** FLO Helm install + OTEL certs via Helm Go SDK ([e5f7ffe](https://github.com/JLCode-tech/awsbnkctl/commit/e5f7ffe47062548c1681de941355ca4690c99578))
* **slice-07:** full BNK activation — IRSA OIDC + secondary ENIs + CNEInstance + License + CWC heal + 20-min polling ([#15](https://github.com/JLCode-tech/awsbnkctl/issues/15)) ([2523e1b](https://github.com/JLCode-tech/awsbnkctl/commit/2523e1bf81d9824f2fa7dee70948c416530fb2a8))
* **slice-08:** EBS CSI driver + gp3 StorageClass + hugepages-ds ([#19](https://github.com/JLCode-tech/awsbnkctl/issues/19)) ([1e6ee07](https://github.com/JLCode-tech/awsbnkctl/commit/1e6ee072121a54ab203e8af7b89ac8f9166ec5aa))
* **slice-10:** host-device data plane (SelfIPs + F5SPKVlan + GatewayClass + IRSA routes) ([#23](https://github.com/JLCode-tech/awsbnkctl/issues/23)) ([23f78eb](https://github.com/JLCode-tech/awsbnkctl/commit/23f78eb285ae104059f1cdf93601d35889ff793c))
* **slice-11:** TMM SIGSEGV fixes + bnk resync helper + scenarios PRD — HTTP 200 e2e on syd-tracer ([#24](https://github.com/JLCode-tech/awsbnkctl/issues/24)) ([b786bd8](https://github.com/JLCode-tech/awsbnkctl/commit/b786bd8779b5dc7a9c94c3da9fe2da9465cb0a3f))
* **slice-12:** jumphost phase + forge aws_profile + awsbnkctl test traffic ([#25](https://github.com/JLCode-tech/awsbnkctl/issues/25)) ([0e23a85](https://github.com/JLCode-tech/awsbnkctl/commit/0e23a8507feb83fd5a93862f6902c53555affb91))
* **slice-13:** scenarios framework + http-routing-e2e + 5 slice-12 cold-start fixes ([#26](https://github.com/JLCode-tech/awsbnkctl/issues/26)) ([623f361](https://github.com/JLCode-tech/awsbnkctl/commit/623f361e8b4711ebf76259e8242acc3c54011bc3))
* **status:** add DEMO banner + warn-only TTL notice (Slice A3) ([#70](https://github.com/JLCode-tech/awsbnkctl/issues/70)) ([f32ca63](https://github.com/JLCode-tech/awsbnkctl/commit/f32ca63aa6a6d5e084dd5775de9962aab9d37022))
* **tf:** terraform.applied.tfvars snapshot after successful apply ([0e6ab89](https://github.com/JLCode-tech/awsbnkctl/commit/0e6ab89125860d9de4a9ee81db707faadf432af4))
* **ui:** rocket-launch + SpaceX-landing renderer for up/down --demo ([#87](https://github.com/JLCode-tech/awsbnkctl/issues/87)) ([3b525f9](https://github.com/JLCode-tech/awsbnkctl/commit/3b525f99cfa7876536226ba02b8b35b43bb7941f))
* **ui:** Slice E — rocket-themed launch renderer for `up --demo` (PRD-10) ([#79](https://github.com/JLCode-tech/awsbnkctl/issues/79)) ([dc7fe00](https://github.com/JLCode-tech/awsbnkctl/commit/dc7fe00754c6bfbbd2ab0fe85d1080638f83a507))


### Bug Fixes

* **agentcore-demo:** correct the teardown blockers, and what the first real run found ([1a22619](https://github.com/JLCode-tech/awsbnkctl/commit/1a22619bc90aaa7aadb7767dc7af6653d95df7b5))
* **agentcore-demo:** key the rate limit on caller identity, clip bodies for syslog ([30e62da](https://github.com/JLCode-tech/awsbnkctl/commit/30e62daedfe5f9e3a16faebb456d89519c26ec73))
* **agentcore-demo:** remove an SSO guard that refused almost always ([6689b30](https://github.com/JLCode-tech/awsbnkctl/commit/6689b3075a00d224a960a08254288c2d5bacd505))
* **agentcore-demo:** the 401 is the tool's, not BNK's ([67cb671](https://github.com/JLCode-tech/awsbnkctl/commit/67cb6714acdd4150a12dda27cdb5ada356b77bfb))
* **agentcore-demo:** two bugs in rebuild.sh found by re-reading it ([4fc36ea](https://github.com/JLCode-tech/awsbnkctl/commit/4fc36eac8426958f07d4fa7d0f237becce90b888))
* **ai-inference-e2e:** bounded SSE-probe poll + 20m vLLM readiness gate ([76be6fa](https://github.com/JLCode-tech/awsbnkctl/commit/76be6faf54eff08a6f7bb403880712324dd8850c))
* **ai-inference-e2e:** live-validated vLLM-on-g5.xlarge fixes ([7c00614](https://github.com/JLCode-tech/awsbnkctl/commit/7c006144bd56c65b70b42baf28a6cd7a411fc7c2))
* **aws:** handle InvalidInstanceConnectEndpointId.NotFound error code during EICE teardown ([ffbc3a0](https://github.com/JLCode-tech/awsbnkctl/commit/ffbc3a00f1c28700bd8f7e61e372e6f4d4c56b19))
* **benchmark:** preflight vLLM served model name before aiperf (WS-E1) ([58f9488](https://github.com/JLCode-tech/awsbnkctl/commit/58f9488128585c522a93a9ebc161e8e9f5653c43))
* **benchmark:** send upstream_namespace tag + scheme-tolerant front-end URL (PRD-11) ([6f0ac1e](https://github.com/JLCode-tech/awsbnkctl/commit/6f0ac1eacd2c36b1d34ebf6145ce3c5a2e071d28))
* **ci:** bump helm.sh/helm/v3 to v3.21.0 to clear govulncheck ([2351d52](https://github.com/JLCode-tech/awsbnkctl/commit/2351d5297988bfbd3f9d043499663ad75527bc96))
* **ci:** file-ignore SA1019 for k8sfake.NewSimpleClientset ([e538d06](https://github.com/JLCode-tech/awsbnkctl/commit/e538d06fb2ca73222ac53ab85e70f8edfdcfa564))
* **ci:** gofmt phase10 nosec annotation alignment ([024d9bc](https://github.com/JLCode-tech/awsbnkctl/commit/024d9bcb4f889ba74ea40ed70bb111be2e77e93a))
* **ci:** green PR [#11](https://github.com/JLCode-tech/awsbnkctl/issues/11) — gosec G115 + G304 annotations ([343e408](https://github.com/JLCode-tech/awsbnkctl/commit/343e4081955521609c34997106fdc099d6d5b322))
* **ci:** green PR [#13](https://github.com/JLCode-tech/awsbnkctl/issues/13) — cspell dockerconfigjson + gosec G101 false positive ([7993e0f](https://github.com/JLCode-tech/awsbnkctl/commit/7993e0f8e4d8541524e9c6ce457dd168b6372977))
* **ci:** green PR [#14](https://github.com/JLCode-tech/awsbnkctl/issues/14) — dead test helper + gosec G101 + cspell ([8bf8732](https://github.com/JLCode-tech/awsbnkctl/commit/8bf87326751011484ae3aab3d2ab010747efa72b))
* **ci:** green PR [#8](https://github.com/JLCode-tech/awsbnkctl/issues/8) — staticcheck dead code + gosec + cspell ([d2db8f4](https://github.com/JLCode-tech/awsbnkctl/commit/d2db8f40a27a91d560564bbb9bbb9ac8611ace1a))
* **ci:** track ml.g5.xlarge example change + gosec G101 on REST path ([125a9c0](https://github.com/JLCode-tech/awsbnkctl/commit/125a9c0e348d509d8e10e27fbbd3aeed9dfe5d33))
* **cli:** --var-file relative-path resolution + wire flag through to Plan ([4e2b26d](https://github.com/JLCode-tech/awsbnkctl/commit/4e2b26d4560088aa22ebf0784439220ce530b735))
* **cli:** split --dry-run var so planCmd default does not poison upCmd ([4973bfc](https://github.com/JLCode-tech/awsbnkctl/commit/4973bfcb5f76cc01e05710990bd694d9e248ceb9))
* **cli:** split flagDownDryRun + audit-document cobra shared-var anti-pattern ([77cd418](https://github.com/JLCode-tech/awsbnkctl/commit/77cd41807fb979de9fc13fb4b3fc0ac6cb6f5eab))
* **cli:** status uses the targeted cluster's kubeconfig + unify -f/--config targeting ([#66](https://github.com/JLCode-tech/awsbnkctl/issues/66)) ([ebd3f28](https://github.com/JLCode-tech/awsbnkctl/commit/ebd3f28ea39572cf73a61b17597fe461d1e0ca48))
* **cold-start:** Chunk 1 — Critical fixes C-2/C-4/C-5/C-6/C-7 ([#29](https://github.com/JLCode-tech/awsbnkctl/issues/29)) ([548ae40](https://github.com/JLCode-tech/awsbnkctl/commit/548ae40a593fe40f3ffc9b63ffd2747176458701))
* **cold-start:** Chunk 2 — High findings H-3..H-8 ([#30](https://github.com/JLCode-tech/awsbnkctl/issues/30)) ([f5e97a8](https://github.com/JLCode-tech/awsbnkctl/commit/f5e97a8e19d4b1e9ac8dbacb1099e56728237a49))
* **cold-start:** Chunk 3+4 — Medium/Low cleanup ([#31](https://github.com/JLCode-tech/awsbnkctl/issues/31)) ([37d17c7](https://github.com/JLCode-tech/awsbnkctl/commit/37d17c7f62069f4777e1fb4e5cc4edbc2f950ab8))
* **cold-start:** Finding [#2](https://github.com/JLCode-tech/awsbnkctl/issues/2) — phase 25 budget 12→18 + ResyncCNEInstance kick ([#34](https://github.com/JLCode-tech/awsbnkctl/issues/34)) ([8c08a7b](https://github.com/JLCode-tech/awsbnkctl/commit/8c08a7b6bae2f1488863f4af96ff1a64f81f7ae9))
* **cold-start:** Finding [#3](https://github.com/JLCode-tech/awsbnkctl/issues/3) — Phase 24b DSSM --insecure readiness overlay ([#35](https://github.com/JLCode-tech/awsbnkctl/issues/35)) ([7448043](https://github.com/JLCode-tech/awsbnkctl/commit/7448043563c75d7dc951b2cca0b7ce060e6f96d0))
* **cold-start:** hotfix — reset RESTMapper on NoKindMatch (C-6 follow-up; live-validated) ([#32](https://github.com/JLCode-tech/awsbnkctl/issues/32)) ([1863b55](https://github.com/JLCode-tech/awsbnkctl/commit/1863b557ff9edd70f9defa6d54377faa27a6151e))
* **cold-start:** Phase 24b — patch all 7 --tls scripts (live-validated hotfix) ([#36](https://github.com/JLCode-tech/awsbnkctl/issues/36)) ([5c03f7b](https://github.com/JLCode-tech/awsbnkctl/commit/5c03f7b08265888f858852401257b947aaa4f06d))
* correctly map subnets to their respective AZs for IPAM ([3e23ab1](https://github.com/JLCode-tech/awsbnkctl/commit/3e23ab10b16d338b4643463d9ce3bb64cd485554))
* **doctor:** no-region AWS chain degrades to warning, honoring green-by-default ([02cad33](https://github.com/JLCode-tech/awsbnkctl/commit/02cad33e20f71a8343bb4d0c300834f28895aa7f))
* **down:** tag-discovery teardown — clear SG cross-refs, discover IGW VPC ([3f6ff0a](https://github.com/JLCode-tech/awsbnkctl/commit/3f6ff0a46b09f359aad4ac1fbf0c4501fd77f590))
* **dryrun:** allow nil Bnk block in infra-only dry-run ([5bdf6cf](https://github.com/JLCode-tech/awsbnkctl/commit/5bdf6cf4f92adcdb09cf4ffc8ec9f15227f83e0c))
* **exec:** rename k8sLongLivedKey → k8sLongLivedEnv (kill gitleaks false positive) ([f0ed90f](https://github.com/JLCode-tech/awsbnkctl/commit/f0ed90f8bcd30de0e24a526151452d2d34bbfa26))
* filter Local Zone subnets from EKS control plane creation ([16954fa](https://github.com/JLCode-tech/awsbnkctl/commit/16954fad774ae29663f3fe819bd54a0340285f31))
* **fmt:** gofmt alignment on render_test.go comment columns ([fbb1ad9](https://github.com/JLCode-tech/awsbnkctl/commit/fbb1ad92b90e2c322be11daeb83a566a8db3a98e))
* **forge,jumphost:** correct cluster name on post-apply register + python3 -m pip on AL2023 ([ca1ad95](https://github.com/JLCode-tech/awsbnkctl/commit/ca1ad95ad779feaa8497b2bd0b8ce1adacb76f68))
* **forge,jumphost:** reconcile aiperf integration with real aiperf 0.10.0 CLI ([8fdf516](https://github.com/JLCode-tech/awsbnkctl/commit/8fdf516ff1a54955992756f83818785d09d0d283))
* **forge:** agent capabilities as dict + target-list object decode (WS-E1) ([f84e5ba](https://github.com/JLCode-tech/awsbnkctl/commit/f84e5ba9569be17421f5c1a7a741ec14ef49163f))
* **forge:** down purges the project + by-name discovery when link is lost ([a9b8679](https://github.com/JLCode-tech/awsbnkctl/commit/a9b8679469022bf1d31a4f0a2a35a693e3d13492))
* **forge:** match canonical aiperf transform + link run to config ([0e603a1](https://github.com/JLCode-tech/awsbnkctl/commit/0e603a1d7a6615199d82531270932b4e360121b7))
* **forge:** parse flat MCP create_project/create_cluster response shapes ([652f05b](https://github.com/JLCode-tech/awsbnkctl/commit/652f05b2611f3ac2d9fc7fabccd7e3515ce789e7))
* **gitleaks:** reword HF comment to avoid generic-api-key false positive ([e3073d9](https://github.com/JLCode-tech/awsbnkctl/commit/e3073d99b7f434be79c7622fb6a20d148a72f55a))
* gofmt internal/aws/phases/phase17b_jumphost.go ([c209e4e](https://github.com/JLCode-tech/awsbnkctl/commit/c209e4eba63afc8749f241eb9da3036f6b1608d9))
* **gosec:** G115 suppression on GPU-LT DiskSize int32 conversion ([4994b17](https://github.com/JLCode-tech/awsbnkctl/commit/4994b179956e271e8be3028e3df75fdb5442e4a3))
* **gpu:** v2 review fixes — DiskSize, AZ-filter unification, taint/capacityType validation ([8e3435b](https://github.com/JLCode-tech/awsbnkctl/commit/8e3435b769bbdfee2189685cd792361d90457d1f))
* **idempotency:** guard phase14 FLO upgrade + phase24b DSSM overlay on healthy re-run ([#74](https://github.com/JLCode-tech/awsbnkctl/issues/74)) ([9d80670](https://github.com/JLCode-tech/awsbnkctl/commit/9d8067090645a04f8977fd159889fc4613def1b3))
* ignore historical test credentials flag by gitleaks after history scrub ([c738096](https://github.com/JLCode-tech/awsbnkctl/commit/c73809654d4ab3ebedd395962693d6c7cacb762d))
* **intent:** move to k8s 1.35 everywhere, raise the floor to 1.34 ([f8db6eb](https://github.com/JLCode-tech/awsbnkctl/commit/f8db6eb8878340fdc78cb93640dfd25256c3bbc7))
* **lifecycle:** honor --dry-run on down; stop up --dry-run polluting state ([#83](https://github.com/JLCode-tech/awsbnkctl/issues/83)) ([464d215](https://github.com/JLCode-tech/awsbnkctl/commit/464d215eaa2f04da6ed8de5315c276cadab42195))
* **lint:** staticcheck ST1005 — error string must not end with punctuation ([9c5f0b6](https://github.com/JLCode-tech/awsbnkctl/commit/9c5f0b663742f26d80b7d9887ce5a6caadfa3d76))
* lowercase docker image tags ([ac92700](https://github.com/JLCode-tech/awsbnkctl/commit/ac927005eb1c77ff8db05cc1495c6b2796b61880))
* move agentcore-demo writeup to its example directory ([9d7822b](https://github.com/JLCode-tech/awsbnkctl/commit/9d7822bb8aca0f43c13ebe2f2d3f30929d626154))
* **phase13:** H1 — accept empty CNEInstance status.state when sub-conditions True ([d1a801b](https://github.com/JLCode-tech/awsbnkctl/commit/d1a801b5c803af638de1db3bb8c020e25c2068a3))
* **phase17c:** add idempotent-skip guard when TMM owns secondary ENIs ([#72](https://github.com/JLCode-tech/awsbnkctl/issues/72)) ([c7474da](https://github.com/JLCode-tech/awsbnkctl/commit/c7474daf9ad049d2ab9f37ca8b6892ca19189728))
* **phase23b:** wait for GatewayClass CRD before applying CRs ([0979b0c](https://github.com/JLCode-tech/awsbnkctl/commit/0979b0ceffe0d9d5c8624a9c4de64d469129a810))
* **phase24c:** H4-rev — poll full window, allow up to 2 bounces ([391696c](https://github.com/JLCode-tech/awsbnkctl/commit/391696c548e26e4e6d62928a5b6ca2de6609223f))
* **phase25:** H1 — gate on F5Tmm+CNEController sub-conditions, not rollup Available ([52f639e](https://github.com/JLCode-tech/awsbnkctl/commit/52f639ec3106baa2d6ece2622dffc8af5b85def6))
* re-audit pass — secret-on-argv, destructive-down gate, BIG-IP lifecycle, docs ([c769e3b](https://github.com/JLCode-tech/awsbnkctl/commit/c769e3b5008faeb5067a377cf44f98221a618826))
* remove completely unused SSM policy constant ([6acbe6f](https://github.com/JLCode-tech/awsbnkctl/commit/6acbe6febb1eb4cdab28bd5d02197421f4a474f6))
* remove tracer from test slices ([27e8a23](https://github.com/JLCode-tech/awsbnkctl/commit/27e8a23900b030ffa5aa70f09a87a0ac825f56dc))
* remove unused constants and functions from jumphost phase ([0cac0af](https://github.com/JLCode-tech/awsbnkctl/commit/0cac0aff558088311a20bfb8c5b36cf31180c366))
* **resync:** behaviour-preserving restore + concurrent-edit guards ([3690d0e](https://github.com/JLCode-tech/awsbnkctl/commit/3690d0ed021035719801073c16c9f5abe507a0de))
* **sagemaker:** auto-recover Failed endpoint + pin LMI served model name ([02196e3](https://github.com/JLCode-tech/awsbnkctl/commit/02196e374d13dfe8f884b13cdd6ffe0d4cd4cf88))
* **sagemaker:** wait for execution-role IAM propagation before create (WS-E1) ([b7f580b](https://github.com/JLCode-tech/awsbnkctl/commit/b7f580b2ee9e13944fd77f3e26bc1c2cdba83da7))
* **scenarios,jumphost,forge:** live-validated cycle-4 fixes (7/7 scenarios green) ([4de8725](https://github.com/JLCode-tech/awsbnkctl/commit/4de87258d35a1a6be061b03d19742f03449160ed))
* **scenarios:** force SSA on manifest apply so re-runs don't need clean ([6f97643](https://github.com/JLCode-tech/awsbnkctl/commit/6f976430486b1a129c4affe008988175a26806e0))
* **scenarios:** narrow http-routing-e2e VIP pool + harden L4Route status check ([fc44e04](https://github.com/JLCode-tech/awsbnkctl/commit/fc44e04e09012ffd242fb6d0e827577e0cc0887a))
* **scenarios:** rework external-resource-pool to EndpointSlice (no Pool CR) ([02bd0fe](https://github.com/JLCode-tech/awsbnkctl/commit/02bd0fe478553cf1f07bb10f1f7befd123e25c6e))
* **scenarios:** verifier correctness — Host header, F5BnkGateway GVR, diagram ns ([3365ca8](https://github.com/JLCode-tech/awsbnkctl/commit/3365ca83e9aa3e3728a9e32cfa13602708dedeb9))
* **security:** clear gosec findings in BIG-IP demo code (CI red since [#93](https://github.com/JLCode-tech/awsbnkctl/issues/93)) ([5b2a952](https://github.com/JLCode-tech/awsbnkctl/commit/5b2a9523b75674cf98d8fbed45d697ee6b9ff4fb))
* **security:** unblock govulncheck CI gate ([b67c28d](https://github.com/JLCode-tech/awsbnkctl/commit/b67c28db324afc068f8be6b76d1fa6a6f05be25e))
* **security:** update Go toolchain for crypto/tls CVE ([16a42b7](https://github.com/JLCode-tech/awsbnkctl/commit/16a42b76fa1cdffda3a71b425f63505ef23702c0))
* **slice-01:** dry-run traverses all phases end-to-end ([74e3633](https://github.com/JLCode-tech/awsbnkctl/commit/74e36331c9dbb7e96638876d4c63b7084f7b07c4))
* **slice-03+04:** k8s label colon + REST flat-shape parsing + gosec G115 ([234a66c](https://github.com/JLCode-tech/awsbnkctl/commit/234a66c66fe78ffdb955b0fab93b1f1157be2f21))
* **slice-04:** forge REST third-shape parsing + base64-encoded kubeconfig ([fb65dd0](https://github.com/JLCode-tech/awsbnkctl/commit/fb65dd0261cedc8507b45d2c30d41a4871eb838d))
* **slice-05:** FAR Secret wraps GCP service-account JSON as dockerconfigjson ([5ea56fb](https://github.com/JLCode-tech/awsbnkctl/commit/5ea56fb0819e2d24ec872af088382458c2624602))
* **slice-06:** FLO Deployment name is f5-lifecycle-operator, not chart-subchart ([2a6d49f](https://github.com/JLCode-tech/awsbnkctl/commit/2a6d49f8cac6028a0cfb8e7c1bcf86d6c4e105bb))
* **slice-07/10:** pin node group to data-path AZ for host-device pattern ([#20](https://github.com/JLCode-tech/awsbnkctl/issues/20)) ([900aa43](https://github.com/JLCode-tech/awsbnkctl/commit/900aa435c810334942c3845689336515d5300abf))
* **slice-07:** correct ManifestVersion default to BNK 2.3.0 manifest ([#18](https://github.com/JLCode-tech/awsbnkctl/issues/18)) ([3f5cf60](https://github.com/JLCode-tech/awsbnkctl/commit/3f5cf60fc78380c0f1193b94197aec19c6bd32ea))
* **slice-07:** create f5-cne-system namespace in Phase 12 ([#16](https://github.com/JLCode-tech/awsbnkctl/issues/16)) ([4b1326e](https://github.com/JLCode-tech/awsbnkctl/commit/4b1326eded6e6aed974b7cd5b895d412b4ce7eab))
* **slice-07:** install Multus CNI v4.2.4 in Phase 12 ([#17](https://github.com/JLCode-tech/awsbnkctl/issues/17)) ([f9a3c4b](https://github.com/JLCode-tech/awsbnkctl/commit/f9a3c4b43d51455e95432fa2924eb7927314e8b4))
* **slice-08:** finish hugepages capacity wait + forge upsert ([#21](https://github.com/JLCode-tech/awsbnkctl/issues/21)) ([e4e96dd](https://github.com/JLCode-tech/awsbnkctl/commit/e4e96ddd4feb849ace01f33639ad6d6cba27010f))
* **slice-09:** use AL2023 AMI so secondary ENIs name as ensN ([#22](https://github.com/JLCode-tech/awsbnkctl/issues/22)) ([565b844](https://github.com/JLCode-tech/awsbnkctl/commit/565b844d778bacde606505532f371ada134d8785))
* **test:** satisfy Scenario.Namespace in cli fakes + gofmt phase24c test ([e8056f8](https://github.com/JLCode-tech/awsbnkctl/commit/e8056f8c61dff833b12852052be4d3c4b1db4c0e))

## [Unreleased]

### Added
- **`AWSBNKCTL_SKIP_AUTH=1`** — credential-free dry-run hook. `up --dry-run` and `down --dry-run` can now render the full plan without live AWS credentials. The hook is rejected for live runs; it is only valid together with `--dry-run`.

## v1.0.0 — 2026-08-06

### Changed
- **Documentation rewrite** — modernized and rewrote all READMEs across the project.
- **Repository cleanup** — removed unused/internal references from public tracked state.

### Fixed
- **Local Zone Cluster Creation** — Fixed an issue where EKS `CreateCluster` failed when AWS Local Zone subnets were provided for the control plane. Local Zone subnets are now correctly filtered out from the `CreateCluster` API request while remaining available for worker nodes in the data plane.

## v0.9.0 — 2026-07-19

Finalizes `v0.9.0-rc1`. Everything in the rc plus the pool-member auto-heal daemon, resync safety hardening, doctor green-by-default fixes, and a reproducible security gate.

### Added

- **`bnk resync --watch`** — daemon mode that watches EndpointSlices for the Services referenced by the targeted HTTPRoutes' backendRefs and auto-fires the weight-toggle resync (debounced, default 2s) when one changes. The operator-side mitigation for the upstream cne-controller gap becomes unattended: the VIP self-heals instead of serving HTTP 500 until someone notices. Composable with all target selectors and `--dry-run`.
- **Dependabot** — weekly grouped go.mod updates + GitHub Actions pin updates, keeping the security gate green as upstream fixes are released.
- **Demo experience subsystem** — `awsbnkctl up --demo` provisions the identical cluster as a normal `up`, plus pre-stages a demo client on the jumphost, tags resources with an absolute expiry, and renders a rocket-themed staged launch UI on interactive terminals. Non-TTY and `--no-color` runs fall back to the plain per-phase log byte-for-byte unchanged.
- **`demo {list,run,clean}` command group** — curated audience catalogue alongside the `scenarios` validation suite. The two registries stay disjoint; `demo list` shows the union (demos + Green scenarios) with a `KIND` column.
- **`http2` demo use-case** — proves end-to-end HTTP/2 (h2c) through TMM, asserting both legs (client→TMM wire HTTP/2 + TMM→backend body `HTTP/2.0`) via SSH+EICE curl from the pre-staged jumphost.
- **`diameter` demo use-case** — proves Diameter (RFC 6733) CER→CEA Result-Code 2001 transit across an L4 BNK Gateway, pushing the embedded Python client via `CopyFileViaEICE` and running it via `RunStagingCommands`.
- **`ingress-migration` demo use-case** — runs ingress-nginx, HAProxy, and a BNK Gateway API route side-by-side over one shared backend, so the legacy-ingress → BNK migration path can be compared live before cutover.
- **`bigip-cis` demo use-case** — stands up an external F5 BIG-IP VE fronted by in-cluster CIS (`k8s-bigip-ctlr`) programming a `VirtualServer` — the traditional appliance model BNK replaces. Opt-in via the `bigipVE:` block; admin password supplied out-of-band via `AWSBNKCTL_BIGIP_PASSWORD`.
- **Jumphost staging primitives** — exported `jumphost.RunStagingCommands` + `jumphost.CopyFileViaEICE` that mint+push ephemeral EICE keys internally (no operator key dance), shared by demo use-cases and the demo-client pre-staging phase.
- **VPC CNI prefix delegation (Phase 08b)** — moved before the node group so nodes boot in prefix mode. Eliminates the cold-start hang caused by secondary-ENI asymmetric drop on the EKS CNI.
- **Phase 11b** — EBS CSI managed addon + `gp3` StorageClass + hugepages-2Mi DaemonSet, in front of the BNK install.
- **Phases 17b/c/d** — multi-ENI jumphost provisioning + interface discovery + (under `--demo`) jumphost client pre-staging.
- **Phase 23b** — `F5SPKVlan` + `GatewayClass` for the host-device pattern, completing the TMM data-plane plumbing.
- **Selectable interface patterns** — `pattern: external-only | dual-interface | sriov-external` (`host-device` is the legacy alias for `dual-interface`). `sriov-external` runs TMM's DPDK dataplane over a `vfio-pci`-bound ENA and is experimental.

### Changed

- **Terraform removed entirely.** The production path is now AWS-SDK-only across all phases. The repository no longer carries Terraform sources, lock files, or vendored modules. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the rationale.
- **`-f/--config` flag unification** — `bnk resync` now accepts `-f/--config`; `status` reads the targeted cluster's `state.env` instead of the host default kubeconfig.
- **CNE Instance auto-resync** — `awsbnkctl bnk resync` ships as a first-class subcommand to work around the upstream HTTPRoute pool-member stale bug (see [`docs/upstream-issues/`](docs/upstream-issues/)).
- **`cluster.yaml` validation** — strict YAML parsing (`KnownFields(true)`); unknown top-level fields fail loud rather than silently being ignored.
- **Security gate policy** — the CI govulncheck step now fails only on reachable vulnerabilities with a released fix; reachable findings with no fix published anywhere surface as warnings instead of permanently blocking every PR. govulncheck is pinned (v1.6.0) for reproducibility, and workflow actions moved to Node24-native majors (checkout/setup-go/upload-artifact v7).
- **Dependency bumps** — `containerd` v1.7.30 → v1.7.33 and Go toolchain go1.26.4 → go1.26.5, clearing the four govulncheck findings that had released fixes.
- **Upstream issue report tightened** — `docs/upstream-issues/cne-controller-endpointslice-not-watched.md` gained an expected-vs-actual section, a kubectl-only recovery, a hardened reference fix, and now documents the `--watch` mitigation.

### Fixed

- **`bnk resync` restore is spec-identical and race-safe** — backendRefs that had no explicit weight are restored with a JSON-Patch `remove` (no more permanent `weight: 1` residue), and both toggle patches carry RFC 6902 `test` guards so a concurrent spec edit fails loudly instead of being clobbered.
- **Doctor green-by-default on region-less hosts** — credentials-resolve-but-no-region now degrades to a warning on the `aws credentials` row (downstream `aws *` rows skipped) instead of a `StatusError` exit; `internal/aws` exposes the `ErrRegionEmpty` sentinel.
- **Phase 14 + Phase 24b idempotency** — both phases now skip cleanly on healthy re-runs (FLO Helm upgrade was unconditional; DSSM overlay reverted its own marker). Healthy `up -f <existing>` is now a true no-op for the BNK install path.
- **Phase 17c on TMM-owns-ENI re-runs** — guard added so an `up` re-run against a healthy cluster no longer fails with `MAC not found` when TMM has already claimed the secondary ENIs into its netns.
- **Pool-member stale workaround in scenarios** — `pkg/bnk.ResyncHTTPRoutes` is now wired into every scenario's Verify step (before the data-plane probe) so probes observe a healed pool.

## v0.x

The pre-`v1.0` series is captured in git history. Each `feat()` / `fix()` commit on `main` / `staging` includes a self-contained design note.

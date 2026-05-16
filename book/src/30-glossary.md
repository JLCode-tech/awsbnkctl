# Glossary

Plain-English definitions of the terms used across the book. Project-specific concepts, AWS / EKS surfaces, Kubernetes admission concepts, and the F5 BIG-IP Next networking vocabulary all live here. Entries are deliberately one or two sentences; the deep-dive lives in the linked chapter where applicable.

## A — D

**AWS standard credential chain**
The ordered lookup the aws-sdk-go-v2 default config performs for AWS credentials: env vars → shared-config profile → SSO cached token → EC2 instance role (IMDS) → ECS / EKS container task role → web-identity (IRSA). The chain stops at the first source that yields a non-empty value. See [Chapter 14](./14-credentials-resolver.md#the-aws-standard-credential-chain).

**Backend** (`--backend`)
The execution context for a tool dispatch. One of `local` (os/exec on the host), `docker` (containerised), `k8s` (in-cluster ops pod or Job), or `ssh:<target>` (a registered SSH endpoint). See [Chapter 17](./17-execution-backends.md).

**BNK**
BIG-IP Next for Kubernetes. F5's Kubernetes-native CNF deployment of BIG-IP, made up of FLO (F5 Lifecycle Operator) + CNE Instance + License + CIS. The reason this CLI exists. See [Chapter 1](./01-what-is-bnk.md).

**CIS**
F5's **Container Ingress Services** — the F5 controller that watches Kubernetes Ingress resources and programs the BIG-IP data plane. (Distinct from IBM's Cloud Internet Services product of the same acronym, which is unrelated to BNK on EKS and is not used by `awsbnkctl`.)

**ClusterIP** (k8s)
A `Service` type that gives a Service an internal cluster IP, reachable only from inside the cluster. Used by the throughput suite's `east-west` mode.

**ClusterRole** (k8s)
A cluster-scoped RBAC role granting verbs on resources. The ops pod's least-privilege ClusterRole grants `jobs.create` in `awsbnkctl-test` but not `pods.delete` in `default`.

**CNE Instance**
A Custom Resource defined by FLO. Represents one deployed instance of the BNK data plane (TMM pods + control plane). Sizing is `Small`/`Medium`/`Large`. See [Chapter 10](./10-deploying-bnk-trials.md).

**Cobra**
[`github.com/spf13/cobra`](https://github.com/spf13/cobra) — the Go CLI library `awsbnkctl` is built on. The command tree at [`internal/cli/`](https://github.com/JLCode-tech/awsbnkctl/tree/main/internal/cli) is a cobra command tree.

**S3**
**Amazon Simple Storage Service** — AWS's object store. The BNK supply chain bucket (FAR archive, JWT licence) lives on S3, server-side encrypted with a customer-managed KMS key, and is read by FLO via IRSA. See [Chapter 25](./25-cos-supply-chain.md).

**cred resolver chain**
See *AWS standard credential chain*.

## E — J

**east-west**
Network direction term: traffic between two endpoints **inside** the cluster (pod-to-pod, service-to-service). The throughput suite's `--mode east-west` measures CNI fabric throughput. See [Chapter 22](./22-throughput-testing.md#--mode-east-west).

**Embedded HCL**
The Terraform source tree compiled into the `awsbnkctl` binary via Go's `//go:embed` directive. The default `tf_source` is `embedded`. Rebuilding the binary picks up HCL changes. See [Chapter 31 §"The embedded HCL"](./31-building-from-source.md#the-embedded-hcl).

**`envFrom`** (k8s)
A Pod spec field that references a Secret or ConfigMap and projects all of its keys as environment variables into the container. The k8s backend's ops pod uses `envFrom: secretRef: awsbnkctl-aws-creds` to receive AWS credentials without listing them in the manifest plaintext. Under IRSA (the default once the v1.x retarget of `internal/exec/k8s_install.yaml` lands), the Secret carries empty data and the SA's projected web-identity token replaces the static creds.

**`extra_hosts`**
The workspace config's list of additional URLs to probe under `awsbnkctl test connectivity`. In v1.0 the value is a bare `[]string` of URLs; per-host method/expected-status overrides are deferred. See [Chapter 20](./20-connectivity-testing.md).

**FAR**
**F5 Application Runtime** — the container-image distribution of the BIG-IP Next data plane. FLO pulls FAR images from `repo.f5.com` using the auth key in the S3 supply-chain bucket.

**FLO**
**F5 Lifecycle Operator** — the Kubernetes operator that owns the CNE Instance + License + supporting resources. The control plane piece of BNK. (The acronym sometimes also surfaces as "F5 Logging Operator" — context disambiguates; in this book it always means Lifecycle Operator.)

**FQDN**
Fully Qualified Domain Name — the absolute form of a DNS name ending with a trailing dot (`www.example.com.`).

**FAR auth key**
The credential tarball (`f5-far-auth-key.tgz`) that FLO uses to pull FAR images from `repo.f5.com`. Lives in the S3 supply-chain bucket. Rotated periodically; see [Chapter 25 §"Rotating the FAR archive"](./25-cos-supply-chain.md#rotating-the-far-archive).

**ghcr.io**
GitHub Container Registry — where the `awsbnkctl-tools-*` images are published. The k8s backend pulls from `ghcr.io/JLCode-tech/awsbnkctl-tools-{ops,iperf3}`.

**GSLB**
**Global Server Load Balancing** — DNS-driven traffic management where the answer a name returns depends on the requesting resolver's network vantage. The thing [Chapter 21](./21-dns-testing-gslb.md) is built to validate.

**`--gslb-compare`**
The DNS-probe flag that fans out across all configured backends in parallel and emits a comparison JSON with `gslb_divergence: true|false`. The signature workflow for "is the GSLB rule taking effect". See [Chapter 21 §"The --gslb-compare workflow"](./21-dns-testing-gslb.md#the---gslb-compare-workflow).

**HCL**
HashiCorp Configuration Language — the syntax of Terraform `.tf` files. The upstream HCL is bundled into the binary; see *Embedded HCL*.

**`aws`**
*Two senses.* The AWS CLI binary (which `awsbnkctl exec -- …` passes through to or replaces, depending on the backend). And the YAML block in `config.yaml` (`aws:`) holding region and AWS profile.

**`ImagePullBackOff`** (k8s)
A Pod status indicating the image couldn't be pulled from the registry. Usually a network or auth problem; sometimes a tag-doesn't-exist problem. See [Chapter 26 §"ImagePullBackOff…"](./26-troubleshooting.md#symptom-imagepullbackoff-on-the-ops-pod-or-throughput-pod).

**`iperf3 mode`** (`--mode`)
The throughput-suite flag selecting `north-south` (LoadBalancer Service, client outside the cluster) or `east-west` (ClusterIP Service, client inside the cluster). See [Chapter 22 §"The two modes"](./22-throughput-testing.md#the-two-modes).

**JWT**
**JSON Web Token** — the signed-token format BNK uses for the subscription licence (`trial.jwt` in the S3 supply-chain bucket).

## I — N

**IMDS**
**Instance Metadata Service** — the link-local `169.254.169.254` endpoint EC2 instances use to read their attached IAM role's short-lived credentials. The fourth source in the AWS standard credential chain.

**IRSA**
**IAM Roles for Service Accounts** — AWS's mechanism for binding an IAM role to a Kubernetes ServiceAccount via the cluster's OIDC provider. A pod running under an IRSA-annotated SA gets its credentials by trading the SA's projected web-identity token at the AWS STS endpoint, with no static API key needed on cluster. FLO authenticates against S3 (FAR-image + JWT-licence reads) via IRSA; the v1.x ops-pod retarget does the same for the `awsbnkctl-ops` SA. See [Chapter 25 §"IRSA trust chain"](./25-cos-supply-chain.md#irsa-trust-chain).

**`k`** (`awsbnkctl k <verb>`)
The internalised kubectl subtree. `awsbnkctl k get/apply/describe/delete/exec/logs/port-forward` — built on `k8s.io/client-go` directly so no host `kubectl` binary is required. See [Chapter 24](./24-day-2-ops.md).

**`kubeconfig`**
The Kubernetes client-configuration file (clusters, contexts, credentials). Defaults to `~/.kube/config`. `awsbnkctl up` auto-fetches the admin kubeconfig post-apply.

**LoadBalancer** (k8s Service type)
A Service type that provisions an external endpoint (a cloud LB on managed Kubernetes; an external IP on bare-metal CNI). Used by the throughput suite's `north-south` mode and by BNK's exposed VIPs.

**Long-lived ops pod**
The k8s backend's persistent execution context. Deployed by `awsbnkctl ops install`; subsequent `--backend k8s` dispatches `kubectl exec` into the same pod rather than starting a fresh Pod each call. Contrasted with the *one-shot Job pattern* used for iperf3 and DNS probes. See [Chapter 19](./19-in-cluster-ops-pod.md).

**Manifest version** (`f5_bigip_k8s_manifest_version`)
The version pin on the f5-bigip-k8s-manifest Helm chart. Transitively pins both the FLO and CIS versions (both are extracted from the manifest chart). See [Chapter 13](./13-terraform-variables.md).

**mdBook**
[rust-lang/mdBook](https://rust-lang.github.io/mdBook/) — the static-site generator the book is built with. Markdown source under `book/src/`, HTML output under `book/book/`. See [Chapter 31 §"The book build"](./31-building-from-source.md#the-book-build).

**miekg/dns**
[github.com/miekg/dns](https://github.com/miekg/dns) — the Go DNS library the GSLB probe is built on. Same library CoreDNS uses; gives `awsbnkctl test dns` full record-type coverage and per-query server selection. See [Chapter 21 §"The awsbnkctl test dns flag surface"](./21-dns-testing-gslb.md#the-awsbnkctl-test-dns-flag-surface).

**north-south**
Network direction term: traffic crossing the cluster boundary — from outside the cluster *to* a pod inside, or vice versa. The throughput suite's `--mode north-south` measures inbound LoadBalancer-path throughput. See [Chapter 22](./22-throughput-testing.md#--mode-north-south).

**NXDOMAIN**
DNS response code indicating "this name does not exist". `awsbnkctl test dns` against a non-existent name exits 1 with rcode=`NXDOMAIN`.

## O — R

**`--on <target>`**
The persistent CLI flag dispatching an `aws`/`exec`/`shell`/`kubectl`/`oc` passthrough over SSH to a named target instead of running it locally. The other half of the SSH-client + `--on` feature alongside the *SSH backend*. See [Chapter 16](./16-on-flag-ssh-jumphosts.md).

**OpenShift**
Red Hat's enterprise Kubernetes distribution. AWS's managed-OpenShift offering is **ROSA** (Red Hat OpenShift Service on AWS); `awsbnkctl` does not target ROSA — it targets stock EKS. The roksbnkctl upstream targeted IBM's managed-OpenShift offering (ROKS) instead. Mentions of OpenShift in this book are either fork-relationship context or apply to the OpenShift `SecurityContextConstraints` admission shape that some inherited Kubernetes manifests still satisfy as a side-effect of also satisfying EKS's Pod Security Admission.

**Ops pod**
Shorthand for the long-lived k8s-backend execution pod deployed in the `awsbnkctl-ops` namespace by `awsbnkctl ops install`. See [Chapter 19](./19-in-cluster-ops-pod.md).

**`passthrough`**
A command that proxies its argv to an underlying tool. `awsbnkctl exec -- …` passes through to the `aws` CLI; `awsbnkctl kubectl …` passes through to `kubectl`. Passthroughs run on whatever backend is selected (local by default).

**PRD**
**Product Requirements Document**. The project uses numbered PRDs under [`docs/prd/`](https://github.com/JLCode-tech/awsbnkctl/tree/main/docs/prd) to coordinate larger feature work. See [Chapter 32 §"The PRD process"](./32-extending-roksbnkctl.md#the-prd-process).

**`PHASE_FROM=`**
The env-var resume mechanism on the e2e driver scripts. `PHASE_FROM=L ./scripts/e2e-test-backends.sh` fast-forwards past phases A-K. See [Chapter 23 §"Resuming a partial run"](./23-e2e-test-plan.md#resuming-a-partial-run).

**RBAC**
**Role-Based Access Control** — the Kubernetes authorization model. The ops pod has a least-privilege RBAC binding; see *ClusterRole*.

**`restricted`** (PSA profile) / **`restricted-v2`** (SCC)
The most-restrictive built-in admission profile. On EKS this is the PSA `restricted` profile applied via the `pod-security.kubernetes.io/enforce: restricted` namespace label; on OpenShift the equivalent is the `restricted-v2` SCC. Both reject pods that run as root, allow privilege escalation, or hold the `ALL` capability set. All `awsbnkctl`-managed pods (ops pod, iperf3 server, DNS probe Job) are written to satisfy both. See [Chapter 22 §"The bundled image and the runAsNonRoot constraint"](./22-throughput-testing.md#the-bundled-image-and-the-runasnonroot-constraint).

**redactor**
The output-stream wrapper at [`internal/exec/redact.go`](https://github.com/JLCode-tech/awsbnkctl/blob/main/internal/exec/redact.go) that masks AWS credential values (access key ID, secret access key, session token) in any subprocess's stdout/stderr before they reach the user's terminal or the log. The defence-in-depth net for credential leaks. See [Chapter 14 §"The redactor"](./14-credentials-resolver.md#the-redactor).

**EKS**
**Amazon Elastic Kubernetes Service** — AWS's managed Kubernetes (not OpenShift). The cluster `awsbnkctl up` provisions, with a self-managed SR-IOV node group on `c5n`/`m5n`-family instances layered on top. See [Chapter 2](./02-why-roks.md).

**`runAsNonRoot`**
A Pod / container `securityContext` field. Required `true` by both the EKS PSA `restricted` profile and the OpenShift `restricted-v2` SCC. Images that have `USER root` in the Dockerfile fail admission with this set.

**RTT**
**Round-Trip Time** — measured in milliseconds for each DNS query. `awsbnkctl test dns -o json` surfaces p50/p95/p99 across the run.

## S — Z

**Schematic JSON**
The deployer-rendered JSON document describing a BNK deployment. Lives in the S3 supply-chain bucket; not consumed at install time, kept for forensics.

**PSA** (Pod Security Admission)
EKS 1.25+'s built-in pod-admission policy. The `awsbnkctl-test` and `awsbnkctl-ops` namespaces carry the `pod-security.kubernetes.io/enforce: restricted` label, which rejects pods that run as root, allow privilege escalation, or hold the `ALL` capability set. The same `securityContext` fields that satisfy PSA also satisfy OpenShift's `restricted-v2` SCC — convenient for the inherited manifests. See [Chapter 22 §"The bundled image and the runAsNonRoot constraint"](./22-throughput-testing.md#the-bundled-image-and-the-runasnonroot-constraint).

**SCC** (legacy)
**Security Context Constraint** — OpenShift's pod-admission policy. EKS uses *PSA* (Pod Security Admission) instead; SCC is included here only because some inherited manifests carry SCC-shaped framing in comments. The `awsbnkctl`-managed pods are written to satisfy both the EKS PSA `restricted` profile and (incidentally) the OpenShift `restricted-v2` SCC.

**Secret** (k8s)
A namespaced resource holding key/value data, typically base64-encoded credentials. The k8s backend creates `awsbnkctl-aws-creds` in the `awsbnkctl-ops` namespace at `ops install` time; under the IRSA-default install path (once the v1.x retarget of `internal/exec/k8s_install.yaml` lands), the Secret is created with empty data and the SA's projected web-identity token replaces the static credentials.

**`secretRef`** (k8s)
The Pod spec form that references a Secret for environment-variable projection. Used together with `envFrom` for the ops pod's credential injection.

**Service** (k8s sense)
A Kubernetes resource that provides a stable endpoint for accessing one or more Pods. Types: `ClusterIP` (default), `NodePort`, `LoadBalancer`, `ExternalName`. See *ClusterIP*, *LoadBalancer*.

**SPDY**
**Speedy** (protocol). The websocket-like, multiplexed-stream protocol Kubernetes uses for `exec` and `port-forward`. `awsbnkctl k exec` is a SPDY client implementation on top of `k8s.io/client-go`'s SPDY executor.

**SSH backend**
The `--backend ssh:<target>` execution path. Runs the tool on a registered SSH endpoint via the [`internal/remote.Client`](https://github.com/JLCode-tech/awsbnkctl/blob/main/internal/remote/ssh.go) wrapper. See [Chapter 17 §"SSH backend"](./17-execution-backends.md#ssh-backend).

**TGW**
**Transit Gateway** — AWS's VPC-to-VPC connectivity service. `awsbnkctl`'s bundled HCL does not provision a TGW by default — the bastion EC2 instance lives in the cluster's VPC (a public subnet) and reaches the cluster's internal endpoints directly. TGW is supported as an optional input for multi-VPC topologies and is documented in the testing module's variables.

**`tfvars`** (`terraform.tfvars`)
Variable-value file for Terraform — assigns concrete values to the HCL's `variable` blocks. `awsbnkctl` auto-renders one from `config.yaml`; user overrides layer on top via `terraform.tfvars.user` and `--var-file`. See [Chapter 13](./13-terraform-variables.md).

**`tf_source`**
The workspace `config.yaml` block selecting where the Terraform source comes from: `embedded` (compiled into the binary; the default), `github` (downloaded tarball), `local` (an on-disk directory). See [Chapter 12 §"tf_source:"](./12-workspace-config.md#tf_source).

**TLS** (`--insecure`)
Transport Layer Security. The `--insecure` flag on `awsbnkctl test connectivity` skips TLS certificate validation for every probe in the run (session-wide, not per-host).

**TMM**
**Traffic Management Microkernel** — the BIG-IP data-plane process. BNK runs TMM as a Pod; the CNE Instance specifies how many and at what size.

**TOFU**
**Trust On First Use** — the SSH-style host-key acceptance pattern. On first connection to a new SSH target, `awsbnkctl` prompts the user to verify the fingerprint; subsequent connections check against the saved fingerprint in `~/.awsbnkctl/known_hosts`. A fingerprint mismatch refuses to connect. See [Chapter 16 §"Host-key handling"](./16-on-flag-ssh-jumphosts.md).

**Trusted Profile** (legacy / upstream)
The IBM IAM construct that the upstream `roksbnkctl` used to let a Kubernetes ServiceAccount assume IBM Cloud permissions. The AWS equivalent — and the mechanism `awsbnkctl` uses — is **IRSA** (IAM Roles for Service Accounts); see that entry. Mentions of Trusted Profile in this book are fork-relationship context or inherited prose pending the v1.x ops-pod retarget.

**TTL**
**Time To Live** — DNS-record cache duration in seconds. `awsbnkctl test dns -o json` surfaces each answer's TTL.

**v1.0**
The release this book is the launch deliverable for. All E2E phases pass on a clean dev box; doctor green-by-default with terraform-only required.

**VPC endpoint**
AWS's private-network access point for managed services (gateway endpoints for S3 / DynamoDB; interface endpoints for everything else). The bundled HCL provisions S3 and STS interface endpoints in the cluster's VPC so the ops pod and node-group pods can reach those services without egressing through the NAT gateway. (The upstream `roksbnkctl` used the equivalent IBM Cloud "VPE" — Virtual Private Endpoint — primitive; the name doesn't carry on AWS.)

**VPC**
**Virtual Private Cloud** — AWS's network-isolation primitive. The cluster's VPC spans at least two AZs (an EKS requirement) and houses the cluster, the SR-IOV node group, and the bastion EC2 instance. Multi-VPC topologies (e.g., a separate testing-client VPC connected via TGW) are supported but not the default. See [Chapter 33 — The data-plane decision](./33-data-plane-decision.md).

**EC2 instance**
AWS's general-purpose VM. The bastion (the `--on jumphost` target) is an EC2 instance; the SR-IOV node group is built from EC2 instances in the `c5n` / `m5n` families.

**workspace**
A named slot under `~/.awsbnkctl/<name>/` containing one `config.yaml`, one Terraform state directory, and (usually) one kubeconfig. The kubectl-style multi-environment isolation primitive. See [Chapter 6](./06-workspaces.md).

**`ws` / `workspace`**
The CLI subtree managing workspaces. `awsbnkctl ws new/use/list/delete`.

## Cross-references

- [Chapter 1](./01-what-is-bnk.md) — BNK context.
- [Chapter 2](./02-why-roks.md) — EKS context.
- [Chapter 14](./14-credentials-resolver.md) — credentials terminology.
- [Chapter 17](./17-execution-backends.md) — backend terminology.
- [Chapter 21](./21-dns-testing-gslb.md) — DNS / GSLB terminology.
- [Chapter 22](./22-throughput-testing.md) — throughput / SCC terminology.

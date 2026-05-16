# syntax=docker/dockerfile:1
# awsbnkctl-tools-aws — Alpine-based tools image carrying:
#   - awscli v2 (official AWS bundle — NOT `pip install awscli` which is v1)
#   - kubectl  (matched to PRD 07 cluster_version default 1.30)
#   - helm     (v3.x — used by Sprint 2's S3 supply-chain manifests)
#   - iperf3 / dig / jq / curl / ca-certificates (runtime deps)
#
# Consumed by the PRD 03 docker execution backend's AWS code paths
# (Sprint 4) — `awsbnkctl up cluster --backend docker` shells into this
# image to run `aws eks describe-cluster`, `kubectl apply`, `helm
# install`, etc. without requiring host installs.
#
# Sprint 0 carry-over (issue_sprint0_validator.md Issue 2): the inherited
# tools-image was IBM-shaped (`tools/docker/ibmcloud/Dockerfile`,
# deleted in Sprint 0). This is its AWS-flavoured replacement. The uid
# 1000 + writable /home/runner + ENV HOME=/home/runner posture is
# directly carried over from roksbnkctl v1.2.0's post-mortem fix for
# `mkdir /.bluemix: permission denied`-style failures (CLI tools that
# expect to write $HOME at first invocation will explode if HOME is
# unset or unwritable).
#
# Multi-stage:
#   Stage 1 (downloader) — fetches awscli v2 zip, kubectl + helm
#   binaries from official URLs; verifies sha256 checksums. Build fails
#   fast if any checksum mismatches (supply-chain protection — a swap
#   at the upstream CDN can't pass through the build).
#
#   Stage 2 (runtime)    — minimal alpine; installs runtime deps, copies
#   binaries from stage 1, drops privileges to uid 1000.

# ── Stage 1: downloader ─────────────────────────────────────────────
FROM alpine:3.19 AS downloader

# Pinned versions. Bumps land in a deliberate commit — supply-chain
# rotations need a code review pass, never a silent CI auto-bump.
ARG AWSCLI_VERSION=2.17.52
ARG KUBECTL_VERSION=v1.30.5
ARG HELM_VERSION=v3.15.4

# Pinned sha256 checksums for the binaries above. Refreshed when
# AWSCLI_VERSION / KUBECTL_VERSION / HELM_VERSION are bumped; mismatch
# fails the build.
#
# Sources:
#   awscli:  https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWSCLI_VERSION}.zip.sha256
#   kubectl: https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256
#   helm:    https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz.sha256sum
ARG AWSCLI_SHA256=11dd0016d93a2e9b5b0f3f13a5d4ee42a09581ef0371461c1c1c1d40c511d81e
ARG KUBECTL_SHA256=b8aa921a580c3d8ba473236815de5ce5173d6fbfa2ccff453fa5eef46cc5ee7a
ARG HELM_SHA256=11400fecfc07fd6f034863e4e0c4c4445594673fd2a129e701fe41f31170cfa9

RUN apk add --no-cache curl unzip tar

WORKDIR /staging

# awscli v2 — vendored python runtime, no host pip needed. The
# `aws-cli` zip extracts to ./aws/ with an installer script; we run
# `./install` into /opt/aws-cli so stage 2 can copy a single tree.
RUN set -eux; \
    curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWSCLI_VERSION}.zip" \
        -o awscli.zip; \
    echo "${AWSCLI_SHA256}  awscli.zip" > awscli.zip.sha256; \
    sha256sum -c awscli.zip.sha256 || { \
        echo "FATAL: awscli checksum mismatch — supply-chain swap suspected"; \
        echo "expected: ${AWSCLI_SHA256}"; \
        echo "got:      $(sha256sum awscli.zip)"; \
        exit 1; }; \
    unzip -q awscli.zip; \
    ./aws/install -i /opt/aws-cli -b /opt/aws-cli/bin; \
    rm -rf awscli.zip aws/

# kubectl — single binary, no install step.
RUN set -eux; \
    curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
        -o /staging/kubectl; \
    echo "${KUBECTL_SHA256}  /staging/kubectl" > kubectl.sha256; \
    sha256sum -c kubectl.sha256 || { \
        echo "FATAL: kubectl checksum mismatch"; \
        exit 1; }; \
    chmod +x /staging/kubectl

# helm — tarball, extract the linux-amd64/helm binary.
RUN set -eux; \
    curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" \
        -o helm.tar.gz; \
    echo "${HELM_SHA256}  helm.tar.gz" > helm.tar.gz.sha256; \
    sha256sum -c helm.tar.gz.sha256 || { \
        echo "FATAL: helm checksum mismatch"; \
        exit 1; }; \
    tar -xzf helm.tar.gz; \
    mv linux-amd64/helm /staging/helm; \
    chmod +x /staging/helm; \
    rm -rf helm.tar.gz linux-amd64/


# ── Stage 2: runtime ────────────────────────────────────────────────
FROM alpine:3.19

LABEL org.opencontainers.image.source="https://github.com/JLCode-tech/awsbnkctl"
LABEL org.opencontainers.image.description="awsbnkctl AWS tools image — awscli v2 + kubectl + helm + iperf3 + dig + jq + curl"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Runtime deps:
#   bash         — awscli v2's wrapper expects it
#   ca-certificates — TLS to AWS endpoints
#   curl, jq     — common HTTP + JSON-munging tools
#   iperf3       — throughput probe surface (PRD 03)
#   bind-tools   — provides `dig` for the DNS probe surface (PRD 03)
#   groff, less  — awscli v2 help pager (awscli v2's vendored python
#                  shells out to `less`; without it `aws help` emits a
#                  confusing error)
#   libstdc++    — awscli v2's vendored python runtime needs it for
#                  some bundled C extensions
RUN apk add --no-cache \
        bash \
        ca-certificates \
        curl \
        jq \
        iperf3 \
        bind-tools \
        groff \
        less \
        libstdc++

# Copy binaries from the downloader stage.
COPY --from=downloader /opt/aws-cli /opt/aws-cli
COPY --from=downloader /staging/kubectl /usr/local/bin/kubectl
COPY --from=downloader /staging/helm /usr/local/bin/helm

# awscli v2's `install -b` flag created /opt/aws-cli/bin/aws as a
# wrapper. Symlink into /usr/local/bin so it's on the default PATH.
RUN ln -s /opt/aws-cli/bin/aws /usr/local/bin/aws \
 && ln -s /opt/aws-cli/bin/aws_completer /usr/local/bin/aws_completer

# Create a non-root runner user with a writable home — uid 1000 matches
# the iperf3 sibling image and the `restricted-v2`-style SCC / k8s
# `runAsNonRoot: true` policy the docker backend hands the image. The
# writable $HOME and `ENV HOME=/home/runner` are the load-bearing piece
# of the v1.2.0 post-mortem (see comment at top of file): tools like
# `aws configure`, `kubectl config set-context`, `helm repo add` all
# expect to write under $HOME at first invocation; if HOME is unset or
# unwritable they explode with `permission denied`.
RUN addgroup -g 1000 runner \
 && adduser -D -u 1000 -G runner -h /home/runner runner \
 && mkdir -p /home/runner/.aws /home/runner/.kube /home/runner/.config/helm \
 && chown -R 1000:1000 /home/runner

USER 1000
WORKDIR /home/runner
ENV HOME=/home/runner
ENV PATH=/home/runner/.local/bin:/usr/local/bin:/usr/bin:/bin

# Default entrypoint left empty — the docker exec backend always
# invokes with an explicit command (`aws ...` / `kubectl ...` /
# `helm ...` / `iperf3 ...`). Setting ENTRYPOINT to any one of them
# would block the multi-tool use case.
CMD ["/bin/bash"]

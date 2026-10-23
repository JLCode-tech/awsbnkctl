# runner.Dockerfile — the "awsbnkctl-tools-runner" image
#
# Published to ghcr.io/jlcode-tech/awsbnkctl-tools-runner and pinned (by digest) from
# the bnkctl-index artifact manifest (tools/awsbnkctl/bnkforge.artifact.json)
# that BNK Forge deploys as a container-runner module. The image carries
# awsbnkctl plus the tools it shells out to (aws-cli, kubectl, helm, git, ssh) and
# runs non-root (uid 1000), as BNK Forge's container engine requires — it refuses
# to start a root image (see the BNK Forge authoring guide §12).
#
# The binary is NOT built here: it is downloaded from the matching GitHub release
# and checksum-verified, so the release tag must already be published (goreleaser
# runs on tag push) BEFORE this image is built.
#
# Build multi-arch directly:
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     --build-arg AWSBNKCTL_VERSION=1.1.0 \
#     -t ghcr.io/jlcode-tech/awsbnkctl-tools-runner:1.1.0 -f runner.Dockerfile --push .
#
# See docs/RELEASE.md for the full release → image → bnkctl-index → BNK Forge chain.

FROM alpine:3.21

# AWSBNKCTL_VERSION is the release tag WITHOUT the leading "v" (e.g. 1.1.0).
# TARGETARCH is supplied automatically by buildx (amd64 / arm64).
ARG AWSBNKCTL_VERSION
ARG TARGETARCH

RUN apk add --no-cache \
    aws-cli \
    kubectl \
    helm \
    git \
    ca-certificates \
    make \
    openssl \
    python3 \
    openssh-client \
    curl \
    tar \
    gzip

# Download + checksum-verify the released awsbnkctl binary for the target arch.
RUN if [ -n "${AWSBNKCTL_VERSION}" ]; then \
      cd /tmp \
      && wget -q "https://github.com/JLCode-tech/awsbnkctl/releases/download/v${AWSBNKCTL_VERSION}/awsbnkctl_${AWSBNKCTL_VERSION}_linux_${TARGETARCH}.tar.gz" \
      && wget -q "https://github.com/JLCode-tech/awsbnkctl/releases/download/v${AWSBNKCTL_VERSION}/checksums.txt" \
      && grep "awsbnkctl_${AWSBNKCTL_VERSION}_linux_${TARGETARCH}.tar.gz" checksums.txt | sha256sum -c - \
      && tar -xzf "awsbnkctl_${AWSBNKCTL_VERSION}_linux_${TARGETARCH}.tar.gz" -C /usr/local/bin awsbnkctl \
      && rm -f /tmp/awsbnkctl_* /tmp/checksums.txt \
      && /usr/local/bin/awsbnkctl version ; \
    fi

# Non-root runner (uid 1000 matches the BNK Forge workspace owner) + the /state
# mount point the artifact manifest declares as the persistent workspace.
RUN adduser -D -u 1000 -h /home/runner runner \
    && mkdir -p /state \
    && chown runner:runner /state

ENV HOME=/home/runner
USER runner
WORKDIR /state

ENTRYPOINT ["awsbnkctl"]
CMD ["--help"]

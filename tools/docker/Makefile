.PHONY: build-iperf3 build-mdbook build-all clean

IMG_PREFIX ?= ghcr.io/JLCode-tech/awsbnkctl-tools
TAG ?= dev

# Repo root, derived relative to this Makefile (tools/docker/Makefile).
# Reserved for future image build stages (Sprint 1's planned `tools-aws`
# image with embedded `aws` / `kubectl` / `eksctl`) that need the repo
# root as build context so they can see ./cmd, ./internal, go.mod.
REPO_ROOT ?= $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../..)

# Build args propagated into image Go build stages — match the host
# `make build` ldflags so any bundled awsbnkctl reports the same
# version / commit / date.
AWSBNKCTL_VERSION ?= $(TAG)
AWSBNKCTL_COMMIT  ?= $(shell git -C $(REPO_ROOT) rev-parse --short HEAD 2>/dev/null || echo none)
AWSBNKCTL_BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Sprint 0 strip: the `build-ibmcloud` target was deleted alongside
# `tools/docker/ibmcloud/` (staff agent's IBM strip). Sprint 1 will add
# an AWS-flavoured replacement target once `tools/docker/aws/` lands.

build-iperf3:
	docker build -t $(IMG_PREFIX)-iperf3:$(TAG) iperf3/

build-mdbook:
	docker build -t $(IMG_PREFIX)-mdbook:$(TAG) mdbook/

build-all: build-iperf3 build-mdbook

clean:
	docker image rm -f $(IMG_PREFIX)-iperf3:$(TAG) $(IMG_PREFIX)-mdbook:$(TAG) || true

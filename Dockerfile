# No `# syntax=` directive on purpose: BuildKit would pull the docker/dockerfile frontend from Docker Hub.
#
# Fetch tiers (SupportTools cluster-services CLAUDE.md "Builds & CI"; cluster-services-1506):
#   - base images: the Harbor Docker Hub proxy cache (a cache we run), tag AND digest pinned
#     (resolved 2026-11-22). The builder's Go minor matches go.mod (`go 1.23.0`, no toolchain line;
#     every dependency declares <= 1.23.0); GOTOOLCHAIN=local never downloads another toolchain.
#   - Go modules: Athens (GOPROXY build-arg; CI passes https://goproxy.support.tools only, a cache we
#     run). The default falls through to proxy.golang.org for off-network local builds. go.sum and
#     the checksum DB verify every module.
#   - apt (runtime packages): the org fetch-cache /ubuntu/ and /ubuntu-security/ prefixes (in-cluster
#     name, resolves inside a buildx RUN on the self-hosted-linux runners). CI passes APT_FALLBACK=none,
#     so a mirror outage fails the build instead of silently fetching upstream. apt verifies every
#     index against the Ubuntu archive keys either way. The shipped image's sources.list is restored.
ARG GO_IMAGE=harbor.support.tools/dockerhub/library/golang:1.23.12-alpine3.22@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f
ARG RUNTIME_IMAGE=harbor.support.tools/dockerhub/library/ubuntu:22.04@sha256:f64b2ffc2f10ab8f8505aa56b2664bd846cbb9df87c7e2532e30f7667021c0e6

FROM ${GO_IMAGE} AS builder

WORKDIR /src

ARG GOPROXY=https://goproxy.support.tools|https://proxy.golang.org
ENV GOPROXY=${GOPROXY} \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Copy go.mod and go.sum files first to leverage Docker cache.
# -x prints every "# get <url>" line, so the build log shows which proxy served each module.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    echo "go module source: GOPROXY=${GOPROXY}" && go mod download -x

COPY . .

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

# -buildvcs=false: the builder has no git (nothing here needs it; the version is stamped by ldflags).
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -buildvcs=false \
    -ldflags "-w -s -X github.com/supporttools/GoSQLGuard/pkg/version.Version=${VERSION} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.GitCommit=${GIT_COMMIT} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.BuildTime=${BUILD_DATE}" \
    -o /gosqlguard . && \
    go build -buildvcs=false \
    -ldflags "-w -s -X github.com/supporttools/GoSQLGuard/pkg/version.Version=${VERSION} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.GitCommit=${GIT_COMMIT} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.BuildTime=${BUILD_DATE}" \
    -o /metadata-recovery ./cmd/metadata-recovery

# Ubuntu runtime: mysqldump 8.0 and pg_dump come from the distro packages.
FROM ${RUNTIME_IMAGE}

ENV DEBIAN_FRONTEND=noninteractive

ARG APT_MIRROR=http://fetch-cache.fetch-cache.svc.a1-ops-prd.local
ARG APT_FALLBACK=upstream

RUN set -eu; \
    src=/etc/apt/sources.list; cp -p "$src" /tmp/sources.list.orig; \
    if [ -n "${APT_MIRROR}" ]; then \
      sed -i -e "s#https\?://archive.ubuntu.com/ubuntu/\?#${APT_MIRROR}/ubuntu/#" \
             -e "s#https\?://security.ubuntu.com/ubuntu/\?#${APT_MIRROR}/ubuntu-security/#" "$src"; \
    fi; \
    if [ -n "${APT_MIRROR}" ] && apt-get update; then \
      echo "apt source: mirror ${APT_MIRROR}"; \
    else \
      [ -z "${APT_MIRROR}" ] || echo "apt source: MIRROR FAILED (${APT_MIRROR})"; \
      if [ "${APT_FALLBACK}" = "none" ]; then echo "APT_FALLBACK=none: refusing upstream" >&2; exit 1; fi; \
      cp -p /tmp/sources.list.orig "$src"; echo "apt source: upstream"; apt-get update; \
    fi; \
    grep -v '^#' "$src" | grep -v '^$'; \
    apt-get install -y --no-install-recommends \
      mysql-client-core-8.0 \
      postgresql-client \
      ca-certificates \
      openssl; \
    cp -p /tmp/sources.list.orig "$src"; rm -f /tmp/sources.list.orig; \
    rm -rf /var/lib/apt/lists/*

# Container metadata
LABEL org.opencontainers.image.title="GoSQLGuard" \
    org.opencontainers.image.description="A tool for automating database backups and retention with support for MySQL and PostgreSQL." \
    org.opencontainers.image.source="https://github.com/supporttools/GoSQLGuard" \
    org.opencontainers.image.vendor="SupportTools" \
    org.opencontainers.image.licenses="MIT"

# Fleet CA standard (operator decision 2026-11-13, cluster-services-1375): NO emSign roots in any file of
# the image. build/noemsign-ca.sh is a vendored copy of SupportTools/cluster-services
# noemsign/noemsign-ca.sh (keep the sha256 identical). strip deselects them in /etc/ca-certificates.conf
# (so a later ca-certificates upgrade keeps them out); guard fails the build on any emSign certificate.
COPY --chmod=0755 build/noemsign-ca.sh /usr/local/sbin/noemsign-ca.sh
RUN /usr/local/sbin/noemsign-ca.sh strip / && /usr/local/sbin/noemsign-ca.sh guard /

# Copy the binaries from the builder stage
COPY --from=builder /gosqlguard /usr/local/bin/gosqlguard
COPY --from=builder /metadata-recovery /usr/local/bin/metadata-recovery

WORKDIR /app

# Create directories for backups and metadata
RUN mkdir -p /app/data/backups /app/data/metadata && chmod -R 755 /app/data

ENTRYPOINT ["/usr/local/bin/gosqlguard"]

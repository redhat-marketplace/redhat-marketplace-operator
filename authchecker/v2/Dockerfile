# syntax = docker/dockerfile:experimental
FROM quay.io/rh-marketplace/golang-base:1.15 as base
ENV PATH=$PATH:/usr/local/go/bin CGO_ENABLED=0

ARG path

WORKDIR /src

COPY go.* ${path}

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    cd ${path} && \
    go mod download

FROM base AS builder
ARG exec
ARG bin
ARG path
ARG TARGETOS=linux
ARG TARGETARCH=amd64

ENV PATH=$PATH:/usr/local/go/bin CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}

RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    cd ${path} && \
    go build \
    -gcflags=-trimpath=$GOPATH -asmflags=-trimpath=$GOPATH \
    -o /app/bin/${bin} ${exec}

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ARG name
ARG bin
ARG app_version
ARG quay_expiration

LABEL name="Red Hat Marketplace ${name}" \
  maintainer="rhmoper@us.ibm.com" \
  vendor="Red Hat Marketplace" \
  release="1" \
  summary="Red Hat Marketplace ${name} Image" \
  description="Container for the Red Hat Marketplace ${name}" \
  version="${app_version}" \
  quay.expires-after=${quay_expiration}

RUN microdnf update --setopt=tsflags=nodocs -y \
  && microdnf clean all \
  && rm -rf /var/cache/yum

ENV USER_UID=1001 \
  USER_NAME=redhat-marketplace-operator \
  BINFILE=${bin}

# install operator binary
COPY --from=builder /app/bin /usr/local/bin
COPY v2/hack/docker/bin/entrypoint /usr/local/bin/entrypoint
COPY v2/hack/docker/bin/user_setup /usr/local/bin/user_setup
COPY LICENSE  /licenses/
RUN  /usr/local/bin/user_setup

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

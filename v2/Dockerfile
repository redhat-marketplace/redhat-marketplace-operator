# syntax = docker/dockerfile:experimental
ARG REGISTRY=quay.io/rh-marketplace

FROM --platform=${BUILDPLATFORM} ${REGISTRY}/golang-base:1.19.1 as base
ARG path
ARG ARCHS

ENV PATH=$PATH:/usr/local/go/bin CGO_ENABLED=0

WORKDIR /src

RUN --mount=type=bind,source=.,target=/src \
    --mount=type=cache,target=/root/.cache/go-build \
    cd ${path} && \
    go mod download

FROM base as builder
ARG path
ARG ARCHS

ENV PATH=$PATH:/usr/local/go/bin CGO_ENABLED=0

WORKDIR /src

RUN --mount=type=bind,source=.,target=/src \
    --mount=type=cache,target=/root/.cache/go-build \
    cd ${path} && \
    BINDIR=/usr/local/go/bin make build

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG name
ARG bin
ARG app_version
ARG path
ARG quay_expiration

LABEL name="Red Hat Marketplace ${name}" \
  maintainer="rhmoper@us.ibm.com" \
  vendor="Red Hat Marketplace" \
  release="1" \
  summary="Red Hat Marketplace ${name} Image" \
  description="Container for the Red Hat Marketplace ${name}" \
  version="${app_version}" \
  quay.expires-after=${quay_expiration}

RUN microdnf install yum procps-ng \
  && yum -y update-minimal --security --sec-severity=Important --sec-severity=Critical \
  && yum clean all \
  && microdnf clean all

ENV USER_UID=1001 \
  USER_NAME=redhat-marketplace-operator \
  BINFILE=${bin}

# install operator binary
COPY v2/hack/docker/bin/entrypoint /usr/local/bin/entrypoint
COPY v2/hack/docker/bin/user_setup /usr/local/bin/user_setup
COPY v2/LICENSE v2/_licenses/* /licenses/
RUN  /usr/local/bin/user_setup

COPY --from=builder /usr/local/go/bin/${bin}-${TARGETOS}-${TARGETARCH} /usr/local/bin/${bin}

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

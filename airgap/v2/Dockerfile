ARG REGISTRY=quay.io/rh-marketplace

FROM ${REGISTRY}/data-service-base:ubi9 AS builder
ARG name
ARG path
ARG exec
ARG bin
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS

ENV CGO_ENABLED=1
ENV GOFLAGS="-buildvcs=false"

WORKDIR /src

RUN --mount=type=bind,source=.,target=/src \
    --mount=type=cache,id=go-build,uid=1001,gid=0,target=/root/.cache/go-build \
    --mount=type=cache,id=mod,uid=1001,gid=0,target=/opt/app-root/src/go/pkg/mod \
    cd ${path} && \
    go build -o /opt/app-root/src/go/bin/${bin} ${exec}

FROM ${REGISTRY}/data-service-ubi-minimal:ubi9
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG name
ARG path
ARG exec
ARG bin
ARG app_version
ARG quay_expiration

LABEL name="Red Hat Marketplace ${name}" \
  maintainer="rhmoper@us.ibm.com" \
  vendor="Red Hat Marketplace" \
  release="1" \
  summary="Red Hat Marketplace ${name} Container Image" \
  description="Container Image for the Red Hat Marketplace ${name}" \
  version="${app_version}" \
  quay.expires-after=${quay_expiration}

ENV USER_UID=1001 \
  USER_NAME=redhat-marketplace-operator \
  BINFILE=/usr/local/bin/${bin}

# install operator binary
COPY v2/hack/docker/bin/entrypoint /usr/local/bin/entrypoint
COPY v2/hack/docker/bin/user_setup /usr/local/bin/user_setup
COPY v2/LICENSE v2/_licenses/* /licenses/
RUN  /usr/local/bin/user_setup

COPY --from=builder /opt/app-root/src/go/bin/${bin} /usr/local/bin/${bin}
COPY --from=builder /opt/app-root/src/go/bin/fips-detect /usr/local/bin/fips-detect
COPY --from=builder /opt/app-root/src/go/bin/grpc-health-probe /usr/local/bin/grpc-health-probe

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

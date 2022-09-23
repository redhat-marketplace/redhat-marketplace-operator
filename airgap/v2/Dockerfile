ARG REGISTRY=quay.io/rh-marketplace

FROM ${REGISTRY}/data-service-base:1.19 as dqlite-lib-builder
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG GO_VERSION
ARG DEBIAN_FRONTEND="noninteractive"
ENV TZ=America/New_York
ENV LD_LIBRARY_PATH=/usr/local/lib
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV DQLITE_VERSION=v1.9.0
ENV RAFT_VERSION=v0.11.2
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH
ENV GO_VERSION=${GO_VERSION} OS=linux ARCH=${TARGETARCH}

RUN apt update -y && \
    apt install -y software-properties-common apt-utils git build-essential dh-autoreconf pkg-config libuv1-dev libsqlite3-dev liblz4-1 liblz4-dev

WORKDIR /opt/airgap

COPY airgap/v2 .

RUN go install -tags libsqlite3 ./cmd/server/main.go

# FROM debian:buster-slim
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ARG name="Data Service"
ARG bin=server
ARG app_version=dev
ARG quay_expiration=7d
ENV LD_LIBRARY_PATH=/usr/local/lib

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
  BINFILE=/opt/main

# COPY --from=dqlite-lib-builder /go/bin /opt/
COPY --from=dqlite-lib-builder /usr/local/lib /usr/local/lib
COPY --from=dqlite-lib-builder /usr/lib /tmp/lib
RUN export UNAME_M=$(uname -m | sed 's/ppc/powerpc/g') && echo $UNAME_M && ls -la /tmp/lib && \
     mkdir -p /usr/lib/${UNAME_M}-${TARGETOS}-gnu && \
     mv /tmp/lib/${UNAME_M}-${TARGETOS}-gnu/libuv.so /usr/local/lib && \
     mv /tmp/lib/${UNAME_M}-${TARGETOS}-gnu/libuv.so.1 /usr/local/lib && \
     mv /tmp/lib/${UNAME_M}-${TARGETOS}-gnu/libuv.so.1.0.0 /usr/local/lib && \
     mv /tmp/lib/${UNAME_M}-${TARGETOS}-gnu/libsqlite3.so /usr/local/lib && \
     mv /tmp/lib/${UNAME_M}-${TARGETOS}-gnu/libsqlite3.so.0 /usr/local/lib && \
     rm -rf /tmp/lib

# install operator binary
COPY v2/hack/docker/bin/entrypoint /usr/local/bin/entrypoint
COPY v2/hack/docker/bin/user_setup /usr/local/bin/user_setup
COPY v2/LICENSE v2/_licenses/* /licenses/
RUN  /usr/local/bin/user_setup

COPY --from=dqlite-lib-builder /go/bin/main /opt/main

# from https://github.com/grpc-ecosystem/grpc-health-probe/releases
# COPY --from=dqlite-lib-builder /bin/grpc_health_probe /bin/grpc_health_probe

# built using latest golang on dqlite-lib-builder
COPY --from=dqlite-lib-builder /go/bin/grpc-health-probe /bin/grpc_health_probe

WORKDIR /usr/local/bin
ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

FROM registry.access.redhat.com/ubi9/go-toolset:1.24.4
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ENV TZ=America/New_York
ENV PATH=$PATH:/opt/app-root/src/go/bin CGO_ENABLED=1
ARG GRPC_HEALTH_VERSION=v0.4.39
ARG DQLITE_VERSION=v1.18.0
ARG LIBUV_VERSION=v1.49.2
ARG quay_expiration=7d

USER 0

# libuv-devel is only available from CRB with subscription, build it ourselves
# dqlite not available on UBI or EPEL, build it ourselves

RUN mkdir -p /opt/app-root/src/go/bin && \
    mkdir -p /opt/app-root/src/go/pkg && \
    dnf update -y && \
    dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm && \
    dnf install -y libsqlite3x-devel libtool pkgconf-pkg-config lz4 lz4-devel && \
    dnf remove epel-release-latest-9 && \
    dnf clean all  && \
    git clone -b $LIBUV_VERSION -v https://github.com/libuv/libuv.git && \
    cd libuv && sh autogen.sh && ./configure --prefix=/usr --libdir=/usr/lib64 && make && make install && \
    cd .. && rm -Rf libuv && \
    git clone -b $DQLITE_VERSION -v https://github.com/canonical/dqlite.git && \
    cd dqlite && autoreconf -i && ./configure --enable-build-raft --prefix=/usr --libdir=/usr/lib64 && make && make install && \
    cd .. && rm -Rf dqlite && \
    go install github.com/grpc-ecosystem/grpc-health-probe@${GRPC_HEALTH_VERSION} && \
    go clean -modcache

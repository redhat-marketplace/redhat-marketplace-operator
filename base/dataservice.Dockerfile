FROM registry.access.redhat.com/ubi8/go-toolset AS dqlite-lib-builder
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS
ENV TZ=America/New_York
ENV PATH=$PATH:/opt/app-root/src/go/bin CGO_ENABLED=1
ARG GRPC_HEALTH_VERSION=v0.4.19
ARG DQLITE_VERSION=v1.15.1
ARG RAFT_VERSION=v0.17.1
ARG LIBUV_VERSION=v1.46.0
ARG quay_expiration=7d

USER 0

# libuv-devel is only available from CRB with subscription, build it ourselves
# raft, dqlite not available on UBI or EPEL, build it ourselves

RUN dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm && \
    dnf install -y libsqlite3x-devel libtool pkgconf-pkg-config lz4 lz4-devel && \
    dnf remove epel-release-latest-8 && \
    dnf clean all  && \
    git clone -b $LIBUV_VERSION -v https://github.com/libuv/libuv.git && \
    cd libuv && sh autogen.sh && ./configure --prefix=/usr --libdir=/usr/lib64 && make && make install && \
    cd .. && rm -Rf libuv && \
    git clone -b $RAFT_VERSION -v https://github.com/canonical/raft.git && \
    cd raft && autoreconf -i && ./configure --prefix=/usr --libdir=/usr/lib64 && make && make install && \
    cd .. && rm -Rf raft && \
    git clone -b $DQLITE_VERSION -v https://github.com/canonical/dqlite.git && \
    cd dqlite && autoreconf -i && ./configure --prefix=/usr --libdir=/usr/lib64 && make && make install && \
    cd .. && rm -Rf dqlite && \
    go install github.com/grpc-ecosystem/grpc-health-probe@${GRPC_HEALTH_VERSION} && \
    go clean -modcache